package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var reloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Reload the XRP daemon configuration (Restart)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Reloading daemon...")
		if err := stopCmd.RunE(cmd, args); err != nil {
			fmt.Println("Could not stop daemon, it might not be running.")
		}
		return startCmd.RunE(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(reloadCmd)
}
