package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

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

		pid, err := strconv.Atoi(string(data))
		if err != nil {
			return fmt.Errorf("invalid PID in file: %w", err)
		}

		process, err := os.FindProcess(pid)
		if err != nil {
			return fmt.Errorf("could not find process: %w", err)
		}

		// Try interrupt first
		if err := process.Signal(os.Interrupt); err != nil {
			// fallback to Kill
			if err := process.Kill(); err != nil {
				return fmt.Errorf("could not stop daemon: %w", err)
			}
		}

		fmt.Println("Daemon stopped successfully.")
		os.Remove(pidFile)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
