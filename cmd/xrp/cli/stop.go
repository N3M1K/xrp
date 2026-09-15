package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/N3M1K/xrp/internal/socket"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the background XRP daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		pidFile := filepath.Join(os.TempDir(), "xrp.pid")
		data, err := os.ReadFile(pidFile)
		if err != nil {
			fmt.Println("Daemon does not appear to be running.")
			return nil
		}

		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			os.Remove(pidFile)
			return fmt.Errorf("invalid PID in file: %w", err)
		}

		if !isProcessRunning(pid) {
			os.Remove(pidFile)
			fmt.Println("Daemon was not running (removed stale PID file).")
			return nil
		}

		// Preferred path: ask the daemon to shut down over IPC. This triggers the
		// same graceful cleanup as a signal and also works on Windows.
		if resp, err := socket.SendWithTimeout(socket.Request{Cmd: "shutdown"}, 3*time.Second); err == nil && resp.Success {
			waitForExit(pid)
			os.Remove(pidFile)
			fmt.Println("Daemon stopped successfully.")
			return nil
		}

		process, err := os.FindProcess(pid)
		if err != nil {
			os.Remove(pidFile)
			return fmt.Errorf("could not find process %d: %w", pid, err)
		}

		// Ask politely first (SIGINT triggers graceful shutdown + cleanup).
		if err := process.Signal(os.Interrupt); err != nil {
			_ = process.Kill()
		}

		// Give it up to ~5s to shut down and clean up hosts entries.
		waitForExit(pid)

		if isProcessRunning(pid) {
			fmt.Println("Daemon did not stop gracefully, forcing...")
			_ = process.Kill()
			time.Sleep(300 * time.Millisecond)
		}

		os.Remove(pidFile)
		fmt.Println("Daemon stopped successfully.")
		return nil
	},
}

// waitForExit polls until the process is gone or ~5s elapse.
func waitForExit(pid int) {
	for i := 0; i < 25; i++ {
		if !isProcessRunning(pid) {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
