package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/N3M1K/halo-proxy/internal/proxy"
	"github.com/N3M1K/halo-proxy/internal/socket"
	"github.com/N3M1K/halo-proxy/internal/ssl"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Halo Proxy daemon in the background",
	RunE: func(cmd *cobra.Command, args []string) error {
		pidFile := filepath.Join(os.TempDir(), "halo.pid")
		pid := 0
		if pidBytes, err := os.ReadFile(pidFile); err == nil {
			if p, err := strconv.Atoi(strings.TrimSpace(string(pidBytes))); err == nil && isProcessRunning(p) {
				pid = p
			} else {
				os.Remove(pidFile) // stale
			}
		}

		if pid != 0 {
			// The daemon is already up. If caddy still lacks the privileged-port
			// capability, grant it and restart so the new caddy picks it up.
			if !ensureCaddyBindCapability() {
				fmt.Println("Daemon is already running.")
				return nil
			}
			fmt.Println("Restarting daemon to apply the new port capability...")
			if resp, err := socket.SendWithTimeout(socket.Request{Cmd: "shutdown"}, 3*time.Second); err == nil && resp.Success {
				waitForExit(pid)
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		fmt.Println("Warming up Halo Proxy daemon environment...")
		resolved, err := runSpinnerUI(ctx)
		if err != nil {
			return fmt.Errorf("failed to provision prerequisites: %w", err)
		}

		// A fresh install downloads caddy during provisioning, so check the
		// privileged-port capability again now that the binary is present.
		ensureCaddyBindCapability()

		// Trust the local mkcert CA once. This is interactive because the very
		// first install on Linux/macOS requires a sudo password. Skip entirely
		// when there is no terminal so scripts don't hang.
		if resolved.Mkcert != "" && stdinIsTerminal() {
			fmt.Println("Ensuring the local development CA is trusted (mkcert -install)...")
			if err := ssl.InstallTrustStore(resolved.Mkcert); err != nil {
				fmt.Printf("⚠️  Could not install the local CA: %v\n", err)
				fmt.Println("   HTTPS will show a certificate warning until you run: mkcert -install")
			}
		}

		exe, err := os.Executable()
		if err != nil {
			return err
		}

		c := exec.Command(exe, "daemon")
		c.SysProcAttr = detachAttr()
		c.Stdin, c.Stdout, c.Stderr = nil, nil, nil
		if err := c.Start(); err != nil {
			return fmt.Errorf("failed to start daemon: %w", err)
		}
		pid = c.Process.Pid
		_ = c.Process.Release()

		if !waitForDaemon(10 * time.Second) {
			fmt.Printf("⚠️  Daemon (PID %d) did not become ready in time.\n", pid)
			printLogTail()
			return nil
		}

		fmt.Printf("Daemon started with PID %d.\n", pid)
		return nil
	},
}

// ensureCaddyBindCapability makes sure caddy can bind ports 80/443 whenever the
// config asks for privileged ports. On Linux this is a one-time
// `sudo setcap cap_net_bind_service=+ep <caddy>`. It returns true only when the
// capability was just granted, so a running daemon can be restarted to use it.
func ensureCaddyBindCapability() bool {
	if os.Geteuid() == 0 {
		return false // root can bind privileged ports without capabilities
	}

	needs, path := proxy.NeedsBindCapability(cfg)
	if !needs {
		return false
	}

	if runtime.GOOS != "linux" {
		fmt.Println("⚠️  caddy must run elevated to bind ports 80/443 (Windows: Administrator, macOS: sudo or use high ports).")
		return false
	}

	if !stdinIsTerminal() {
		fmt.Printf("⚠️  caddy needs permission to bind ports 80/443. Run:\n   sudo setcap cap_net_bind_service=+ep %s\n", path)
		return false
	}

	fmt.Println("caddy needs permission to bind ports 80/443 (one-time setup).")
	if err := proxy.GrantBindCapability(path); err != nil {
		fmt.Printf("⚠️  Could not grant it: %v\n   Run manually: sudo setcap cap_net_bind_service=+ep %s\n", err, path)
		return false
	}
	fmt.Println("✅ Granted cap_net_bind_service to caddy.")
	return true
}

// waitForDaemon polls the IPC socket until the daemon answers.
func waitForDaemon(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if resp, err := socket.SendWithTimeout(socket.Request{Cmd: "status"}, 750*time.Millisecond); err == nil && resp.Success {
			return true
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}

// stdinIsTerminal reports whether stdin is an interactive terminal. This is
// deliberately stricter than ModeCharDevice (which /dev/null also satisfies).
func stdinIsTerminal() bool {
	fd := os.Stdin.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

// printLogTail prints the last few lines of the daemon log to help diagnose a
// failed startup.
func printLogTail() {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return
	}
	data, err := os.ReadFile(filepath.Join(cacheDir, "halo", "halo.log"))
	if err != nil {
		return
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > 10 {
		lines = lines[len(lines)-10:]
	}
	fmt.Println("--- last daemon log lines ---")
	for _, l := range lines {
		fmt.Println(l)
	}
}

func init() {
	rootCmd.AddCommand(startCmd)
}
