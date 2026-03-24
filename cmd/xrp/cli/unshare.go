package cli

import (
	"fmt"

	"github.com/N3M1K/xrp/internal/socket"
	"github.com/spf13/cobra"
)

var unshareCmd = &cobra.Command{
	Use:   "unshare [project]",
	Short: "Stop an active cloudflare tunnel mapped to a local project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project := args[0]
		
		resp, err := socket.Send(socket.Request{
			Cmd:  "unshare",
			Args: map[string]string{"project": project},
		})

		if err != nil {
			return fmt.Errorf("failed to communicate with daemon: %w", err)
		}

		if !resp.Success {
			return fmt.Errorf("failed: %s", resp.Error)
		}

		fmt.Printf("🛑 Tunnel for %s stopped.\n", project)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(unshareCmd)
}
