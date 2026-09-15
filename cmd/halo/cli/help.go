package cli

import (
	"github.com/spf13/cobra"
)

var customHelpCmd = &cobra.Command{
	Use:   "help",
	Short: "Help about any command",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Parent().Help()
	},
}

func init() {
	rootCmd.SetHelpCommand(customHelpCmd)
}
