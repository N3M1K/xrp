package cli

import (
	"fmt"
	"github.com/N3M1K/xrp/internal/config"
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
		fmt.Printf("✅ Project '%s' has been successfully bound to custom domain *.%s\n", project, tld)
		fmt.Printf("⚠️ Please ensure the XRP daemon is running or reload it to apply routing changes.\n")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(setTldCmd)
}
