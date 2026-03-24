package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of XRP",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("XRP v0.5.4")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
