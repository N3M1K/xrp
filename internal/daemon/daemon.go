package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/N3M1K/xrp/internal/config"
	"github.com/N3M1K/xrp/internal/deps"
	"github.com/N3M1K/xrp/internal/hosts"
	"github.com/N3M1K/xrp/internal/proxy"
	"github.com/N3M1K/xrp/internal/scanner"
	"github.com/N3M1K/xrp/internal/socket"
	"github.com/N3M1K/xrp/internal/ssl"
	"github.com/N3M1K/xrp/internal/tunnel"
)

func getPIDFilePath() string {
	return filepath.Join(os.TempDir(), "xrp.pid")
}

func WritePID() error {
	pid := os.Getpid()
	pidFile := getPIDFilePath()
	return os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", pid)), 0644)
}

func RemovePID() {
	pidFile := getPIDFilePath()
	os.Remove(pidFile)
}

func getLogFilePath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	logDir := filepath.Join(cacheDir, "xrp")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(logDir, "xrp.log"), nil
}

func Run(cfg *config.Config) error {
	logFilePath, err := getLogFilePath()
	if err != nil {
		return fmt.Errorf("could not setup log directory: %w", err)
	}
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("could not open log file: %w", err)
	}
	defer logFile.Close()

	logger := log.New(logFile, "[daemon] ", log.LstdFlags)
	logger.Printf("Starting XRP daemon...")
	logger.Printf("HTTP port: %d, HTTPS port: %d", cfg.HTTPPort, cfg.HTTPSPort)

	if err := WritePID(); err != nil {
		return fmt.Errorf("could not write PID file: %w", err)
	}
	defer RemovePID()

	// Provision required system dependencies cleanly and concurrently
	logger.Printf("Ensuring pre-packed dependencies (Caddy, mkcert, cloudflared) orchestrations...")
	if _, err := deps.EnsureAll(context.Background()); err != nil {
		logger.Printf("Warning: partial dependency provisioning failures: %v", err)
	}

	// Dynamically override PATH across child exec routines
	if cacheDir, err := os.UserCacheDir(); err == nil {
		xrpBinDir := filepath.Join(cacheDir, "xrp", "bin")
		os.Setenv("PATH", xrpBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	}

	// Ensure mkcert is ready
	var certPairs []ssl.CertPair
	if err := ssl.CheckMkcert(); err != nil {
		logger.Printf("Warning: mkcert not found, SSL might not work. Please install it.")
	} else {
		logger.Printf("Generating/Verifying mkcert certificates for all configured TLDs")
		if err := ssl.InstallTrustStore(); err != nil {
			logger.Printf("Failed to install mkcert trust store: %v", err)
		}
		pairs, err := ssl.GenerateAllCerts(cfg)
		if err != nil {
			logger.Printf("Warning: partial cert generation failure: %v", err)
		}
		certPairs = pairs
	}

	// Check hosts file writability (this also serves as the admin/elevation check
	// since writing the hosts file on Windows requires Administrator privileges)
	hostsWritable := hosts.IsWritable()
	if !hostsWritable {
		if runtime.GOOS == "windows" {
			logger.Printf("WARNING: XRP is not running as Administrator.")
			logger.Printf("WARNING: Caddy CANNOT bind to ports 80/443 and domains will not resolve.")
			logger.Printf("WARNING: Please restart XRP from an elevated (Administrator) terminal.")
		} else {
			logger.Printf("WARNING: Hosts file is not writable. Domains will not resolve in the browser.")
			logger.Printf("WARNING: Caddy may also fail to bind ports 80/443. Try: sudo setcap cap_net_bind_service=+ep $(which caddy)")
		}
	}

	// Ensure Caddy starts (must happen AFTER hosts writability check so we have the hostsWritable flag)
	if err := proxy.StartCaddy(); err != nil {
		logger.Printf("Failed to start Caddy: %v", err)
	}

	// Start socket server for IPC (CLI & VSCode)
	go func() {
		if err := socket.StartServer(logger); err != nil {
			logger.Printf("Failed to start IPC socket server: %v", err)
		}
	}()

	ticker := time.NewTicker(time.Duration(cfg.PollInterval) * time.Second)
	defer ticker.Stop()
	defer tunnel.StopAll()
	defer func() {
		// Clean up hosts entries on shutdown
		if hostsWritable {
			if err := hosts.RemoveAllEntries(); err != nil {
				logger.Printf("Failed to clean up hosts entries: %v", err)
			} else {
				logger.Printf("Cleaned up hosts file entries")
			}
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	logger.Printf("Daemon running, scanning every %d seconds", cfg.PollInterval)

	for {
		select {
		case sig := <-sigChan:
			logger.Printf("Received signal %s, shutting down...", sig.String())
			proxy.StopCaddy()
			return nil

		case <-ticker.C:
			// Reload config on every tick to pick up TLD changes from CLI/TUI
			if freshCfg, err := config.LoadConfig(); err == nil {
				cfg = freshCfg
				// Always regenerate cert list — GenerateAllCerts skips existing valid certs
				// but MUST return them so certPairs stays populated for Caddy.
				// Bug fix: the old `&& len(pairs) > 0` guard caused certPairs to be
				// cleared when all certs already existed (pairs returned, no new generated).
				if pairs, err := ssl.GenerateAllCerts(cfg); err == nil {
					certPairs = pairs
				}
			}

			processes, err := scanner.ScanProcesses()
			if err != nil {
				logger.Printf("Error scanning processes: %v", err)
				continue
			}

			if len(processes) > 0 {
				logger.Printf("Found %d local development servers. Updating proxy...", len(processes))
			}

			// Map active tunnels
			tunnels := tunnel.GetActiveTunnels()
			for i := range processes {
				if url, found := tunnels[processes[i].ProjectName]; found {
					processes[i].TunnelURL = url
				}
			}

			// Share with socket clients
			socket.UpdateProcesses(processes)

			// Build hostnames list and sync hosts file
			if hostsWritable {
				var hostnames []string
				for _, p := range processes {
					if p.ProjectName != "" {
						tld := cfg.TLD
						if custom, ok := cfg.ProjectTLDs[p.ProjectName]; ok && custom != "" {
							tld = custom
						}
						cleanTld := strings.TrimPrefix(tld, ".")
						hostnames = append(hostnames, fmt.Sprintf("%s.%s", p.ProjectName, cleanTld))
					}
				}
				if err := hosts.SyncEntries(hostnames); err != nil {
					logger.Printf("Failed to sync hosts file: %v", err)
				}
			}

			// Generate and Apply Caddy config
			caddyConfig := proxy.GenerateConfig(processes, cfg, certPairs)
			if err := proxy.ApplyConfig(caddyConfig); err != nil {
				logger.Printf("Failed to apply proxy configuration: %v", err)
			}
		}
	}
}
