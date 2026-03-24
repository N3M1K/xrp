package daemon

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/N3M1K/xrp/internal/config"
	"github.com/N3M1K/xrp/internal/deps"
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
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	logDir := filepath.Join(homeDir, ".local", "share", "xrp")
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

	if err := WritePID(); err != nil {
		return fmt.Errorf("could not write PID file: %w", err)
	}
	defer RemovePID()

	// Provision required system dependencies cleanly and concurrently
	logger.Printf("Ensuring pre-packed dependencies (Caddy, mkcert, cloudflared) orchestrations...")
	if _, err := deps.EnsureAll(); err != nil {
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

	// Ensure Caddy starts (stubbed for now in proxy module)
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

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	logger.Printf("Daemon running, scanning every %d seconds", cfg.PollInterval)

	for {
		select {
		case sig := <-sigChan:
			if sig == syscall.SIGHUP {
				logger.Printf("Received SIGHUP, reloading configuration...")
				if newCfg, err := config.LoadConfig(); err == nil {
					*cfg = *newCfg
				} else {
					logger.Printf("Failed to reload config: %v", err)
				}
				continue
			}

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

			// Generate and Apply Caddy config
			caddyConfig := proxy.GenerateConfig(processes, cfg.TLD, certPath, keyPath)
			if err := proxy.ApplyConfig(caddyConfig); err != nil {
				logger.Printf("Failed to apply proxy configuration: %v", err)
			}
		}
	}
}
