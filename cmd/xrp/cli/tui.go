package cli

import (
	"strings"

	"github.com/N3M1K/xrp/internal/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the interactive Terminal User Interface (TUI)",
	RunE: func(cmd *cobra.Command, args []string) error {
		tld := ".local"
		if cfg != nil && cfg.TLD != "" {
			tld = cfg.TLD
		}
		cleanTld := strings.TrimPrefix(tld, ".")
		return tui.Start(cleanTld)
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
