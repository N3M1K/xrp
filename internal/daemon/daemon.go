package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/N3M1K/halo-proxy/internal/config"
	"github.com/N3M1K/halo-proxy/internal/deps"
	"github.com/N3M1K/halo-proxy/internal/hosts"
	"github.com/N3M1K/halo-proxy/internal/proxy"
	"github.com/N3M1K/halo-proxy/internal/scanner"
	"github.com/N3M1K/halo-proxy/internal/socket"
	"github.com/N3M1K/halo-proxy/internal/ssl"
	"github.com/N3M1K/halo-proxy/internal/tunnel"
)

func getPIDFilePath() string {
	return filepath.Join(os.TempDir(), "halo.pid")
}

func WritePID() error {
	pid := os.Getpid()
	pidFile := getPIDFilePath()
	return os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644)
}

func RemovePID() {
	os.Remove(getPIDFilePath())
}

func getLogFilePath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	logDir := filepath.Join(cacheDir, "halo")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(logDir, "halo.log"), nil
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
	logger.Printf("Starting Halo Proxy daemon (pid %d)...", os.Getpid())
	logger.Printf("HTTP port: %d, HTTPS port: %d, admin port: %d", cfg.HTTPPort, cfg.HTTPSPort, cfg.CaddyPort)

	if err := WritePID(); err != nil {
		return fmt.Errorf("could not write PID file: %w", err)
	}
	defer RemovePID()

	// Provision required system dependencies cleanly and concurrently
	logger.Printf("Ensuring dependencies (Caddy, mkcert, cloudflared)...")
	if _, err := deps.EnsureAll(context.Background()); err != nil {
		logger.Printf("Warning: partial dependency provisioning failures: %v", err)
	}
	logger.Printf("Dependencies ready")

	// Dynamically override PATH across child exec routines
	if cacheDir, err := os.UserCacheDir(); err == nil {
		haloBinDir := filepath.Join(cacheDir, "halo", "bin")
		os.Setenv("PATH", haloBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	}

	// Certificates: the interactive `halo start` command handles installing the
	// mkcert CA into the system trust store. Here we only generate the certs
	// (which never needs elevated privileges).
	// Certificates are generated per discovered hostname inside tick(); here we
	// only verify mkcert is available.
	if err := ssl.CheckMkcert(); err != nil {
		logger.Printf("Warning: mkcert not found, HTTPS will be unavailable: %v", err)
	}

	// Check hosts file writability (this also serves as the admin/elevation check
	// since writing the hosts file on Windows requires Administrator privileges)
	hostsWritable := hosts.IsWritable()
	if !hostsWritable {
		if runtime.GOOS == "windows" {
			logger.Printf("WARNING: hosts file is not writable. Restart Halo Proxy from an elevated (Administrator) terminal for custom TLDs to resolve.")
		} else {
			logger.Printf("WARNING: hosts file is not writable. Only .localhost domains will resolve; custom TLDs need root or a writable HALO_HOSTS_PATH.")
		}
	}

	// Ensure Caddy starts
	if err := proxy.StartCaddy(cfg); err != nil {
		logger.Printf("Failed to start Caddy: %v", err)
	} else {
		logger.Printf("Caddy is running (admin 127.0.0.1:%d)", cfg.CaddyPort)
	}

	// Start socket server for IPC (CLI & VSCode)
	shutdownCh := make(chan struct{})
	var shutdownOnce sync.Once
	socket.SetShutdownHandler(func() { shutdownOnce.Do(func() { close(shutdownCh) }) })
	go func() {
		if err := socket.StartServer(logger); err != nil {
			logger.Printf("Failed to start IPC socket server: %v", err)
		}
	}()

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
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	logger.Printf("Daemon running, scanning every %d seconds", cfg.PollInterval)

	// Perform an immediate first scan so `halo list` is populated right away.
	st := &daemonState{}
	tick(logger, cfg, st, hostsWritable)

	ticker := time.NewTicker(interval(cfg.PollInterval))
	defer ticker.Stop()

	for {
		select {
		case sig := <-sigChan:
			logger.Printf("Received signal %s, shutting down...", sig.String())
			proxy.StopCaddy(cfg)
			return nil

		case <-shutdownCh:
			logger.Printf("Shutdown requested via IPC, shutting down...")
			proxy.StopCaddy(cfg)
			return nil

		case <-ticker.C:
			// Reload config on every tick to pick up TLD changes from CLI/TUI
			if freshCfg, err := config.LoadConfig(); err == nil {
				cfg = freshCfg
			}
			// Apply poll interval changes on the fly.
			ticker.Reset(interval(cfg.PollInterval))

			tick(logger, cfg, st, hostsWritable)
		}
	}
}

func interval(seconds int) time.Duration {
	if seconds < 1 {
		seconds = 1
	}
	return time.Duration(seconds) * time.Second
}

// daemonState carries values that must survive between poll cycles.
type daemonState struct {
	certHash  string
	certPairs []ssl.CertPair
}

// tick performs one scan → enrich → publish → sync cycle.
func tick(logger *log.Logger, cfg *config.Config, st *daemonState, hostsWritable bool) {
	processes, err := scanner.ScanProcesses()
	if err != nil {
		logger.Printf("Error scanning processes: %v", err)
		return
	}

	// Map active tunnels onto the discovered processes.
	tunnels := tunnel.GetActiveTunnels()

	var hostnames []string
	for i := range processes {
		p := &processes[i]
		tld := cfg.EffectiveTLD(p.ProjectName)
		host := fmt.Sprintf("%s.%s", p.ProjectName, tld)
		p.URL = "https://" + host
		if url, found := tunnels[p.ProjectName]; found {
			p.TunnelURL = url
		}
		hostnames = append(hostnames, host)
	}

	if len(processes) > 0 {
		logger.Printf("Found %d local development server(s). Updating proxy...", len(processes))
	}

	// Share with socket clients (CLI, TUI, VSCode)
	socket.UpdateProcesses(processes)

	if hostsWritable {
		if err := hosts.SyncEntries(hostnames); err != nil {
			logger.Printf("Failed to sync hosts file: %v", err)
		}
	}

	// Issue a certificate whose SANs cover exactly the hostnames we discovered.
	// Regenerated only when the set of hostnames changes. On failure we keep the
	// previous certificate so already-working names don't regress.
	if pair, hash, err := ssl.EnsureCertForHosts(hostnames, st.certHash); err != nil {
		logger.Printf("Warning: certificate generation failed: %v", err)
	} else {
		if hash != st.certHash {
			logger.Printf("Issued certificate for %d host(s)", len(hostnames)+1)
		}
		st.certHash = hash
		st.certPairs = []ssl.CertPair{pair}
	}

	caddyConfig := proxy.GenerateConfig(processes, cfg, st.certPairs)
	if err := proxy.ApplyConfig(cfg, caddyConfig); err != nil {
		logger.Printf("Failed to apply proxy configuration: %v", err)
	}
}
