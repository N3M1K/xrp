package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Halo Proxy",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Halo Proxy v0.5.10")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
