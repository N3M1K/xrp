package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the XRP daemon in the background",
	RunE: func(cmd *cobra.Command, args []string) error {
		pidFile := filepath.Join(os.TempDir(), "xrp.pid")
		if pidBytes, err := os.ReadFile(pidFile); err == nil {
			pid, _ := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
			if process, err := os.FindProcess(pid); err == nil {
				// Na Windows FindProcess uspěje vždy, na Unix to kontroluje proces.
				// Použijeme Signal(0) pro zjištění existence procesu:
				if err := process.Signal(syscall.Signal(0)); err == nil {
					fmt.Println("Daemon is already running.")
					return nil
				}
			}
			os.Remove(pidFile) // stale, smazat
		}

		exe, err := os.Executable()
		if err != nil {
			return err
		}

		fmt.Println("Starting XRP daemon...")
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
