package cli

import (
	"github.com/N3M1K/xrp/internal/daemon"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:    "daemon",
	Hidden: true,
	Short:  "Runs the blocking daemon loop (used internally by start)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return daemon.Run(cfg)
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
}
