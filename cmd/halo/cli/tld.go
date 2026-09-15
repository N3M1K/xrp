package cli

import (
	"fmt"
	"strings"

	"github.com/N3M1K/halo-proxy/internal/config"
	"github.com/spf13/cobra"
)

var setTldCmd = &cobra.Command{
	Use:   "set-tld [project] [tld]",
	Short: "Override the default TLD for a specific local project",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		project := args[0]
		tld := args[1]

		if err := config.SetProjectTLD(project, tld); err != nil {
			return fmt.Errorf("failed to save custom TLD: %w", err)
		}

		if strings.Trim(strings.TrimSpace(tld), ".") == "" {
			fmt.Printf("✅ Custom TLD cleared for '%s' (the default '%s' applies).\n", project, cfg.TLD)
		} else {
			fmt.Printf("✅ Project '%s' is now served at https://%s.%s\n", project, project, config.NormalizeTLD(tld))
		}
		fmt.Println("⚠️  The daemon applies this within one poll cycle (~5s).")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(setTldCmd)
}
