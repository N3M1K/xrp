package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
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
	logger.Printf("Admin mode: %v, HTTP port: %d, HTTPS port: %d", config.IsAdmin(), cfg.HTTPPort, cfg.HTTPSPort)

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
	if err := ssl.CheckMkcert(); err != nil {
		logger.Printf("Warning: mkcert not found, SSL might not work. Please install it.")
	} else {
		logger.Printf("Generating/Verifying mkcert certificates for *.%s", cfg.TLD)
		if err := ssl.InstallTrustStore(); err != nil {
			logger.Printf("Failed to install mkcert trust store: %v", err)
		}
	}

	certPath, keyPath, err := ssl.GenerateCert(cfg.TLD)
	if err != nil {
		logger.Printf("Failed to generate certificates: %v", err)
	}

	// Ensure Caddy starts
	if err := proxy.StartCaddy(); err != nil {
		logger.Printf("Failed to start Caddy: %v", err)
	}

	// Check hosts file writability
	hostsWritable := hosts.IsWritable()
	if !hostsWritable {
		logger.Printf("WARNING: Hosts file is not writable. Domains will not resolve in the browser. Run as admin or manually add entries to the hosts file.")
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
				cleanTld := strings.TrimPrefix(cfg.TLD, ".")
				var hostnames []string
				for _, p := range processes {
					if p.ProjectName != "" {
						hostnames = append(hostnames, fmt.Sprintf("%s.%s", p.ProjectName, cleanTld))
					}
				}
				if err := hosts.SyncEntries(hostnames); err != nil {
					logger.Printf("Failed to sync hosts file: %v", err)
				}
			}

			// Generate and Apply Caddy config
			caddyConfig := proxy.GenerateConfig(processes, cfg.TLD, certPath, keyPath, cfg.HTTPPort, cfg.HTTPSPort)
			if err := proxy.ApplyConfig(caddyConfig); err != nil {
				logger.Printf("Failed to apply proxy configuration: %v", err)
			}
		}
	}
}
