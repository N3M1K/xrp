package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/N3M1K/xrp/internal/socket"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the status of the XRP daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := socket.Send(socket.Request{Cmd: "status"})
		if err != nil {
			fmt.Printf("%s%sXRP daemon is NOT running. Start it with 'xrp start'.%s\n", Bold, Red, Reset)
			return nil
		}
		
		if !resp.Success {
			fmt.Printf("%s%sStatus check failed: %s%s\n", Bold, Red, resp.Error, Reset)
			return nil
		}

		var statusStr string
		json.Unmarshal(resp.Data, &statusStr)

		pidFile := filepath.Join(os.TempDir(), "xrp.pid")
		data, _ := os.ReadFile(pidFile)

		fmt.Printf("%s%sDaemon is %s (PID: %s).%s\n", Bold, Green, statusStr, string(data), Reset)
		
		// Run list command afterwards
		listCmd.RunE(cmd, args)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
