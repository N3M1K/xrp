package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the XRP daemon in the background",
	RunE: func(cmd *cobra.Command, args []string) error {
		pidFile := filepath.Join(os.TempDir(), "xrp.pid")
		if pidBytes, err := os.ReadFile(pidFile); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes))); err == nil {
				if isProcessRunning(pid) {
					fmt.Println("Daemon is already running.")
					return nil
				}
			}
			os.Remove(pidFile) // stale, smazat
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		fmt.Println("Warming up XRP daemon environment...")
		if err := runSpinnerUI(ctx); err != nil {
			return fmt.Errorf("failed to provision prerequisites: %w", err)
		}

		exe, err := os.Executable()
		if err != nil {
			return err
		}

		c := exec.Command(exe, "daemon")
		if err := c.Start(); err != nil {
			return fmt.Errorf("failed to start daemon: %w", err)
		}

		fmt.Printf("Daemon started with PID %d.\n", c.Process.Pid)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
