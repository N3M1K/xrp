package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/N3M1K/xrp/internal/scanner"
	"github.com/N3M1K/xrp/internal/socket"
	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"
)

var shareCmd = &cobra.Command{
	Use:   "share [project]",
	Short: "Expose a local project securely to the internet via Cloudflare Tunnel",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project := args[0]
		
		respList, err := socket.Send(socket.Request{Cmd: "list"})
		if err != nil || !respList.Success {
			return fmt.Errorf("daemon is not running or error: %v", err)
		}

		var processes []scanner.Process
		json.Unmarshal(respList.Data, &processes)

		var targetPort int
		for _, p := range processes {
			if p.ProjectName == project {
				targetPort = p.Port
				break
			}
		}

		if targetPort == 0 {
			return fmt.Errorf("project '%s' not found in running services", project)
		}

		fmt.Printf("Starting secure tunnel for %s (Port %d)...\n", project, targetPort)
		
		resp, err := socket.Send(socket.Request{
			Cmd:  "share",
			Args: map[string]string{"project": project, "port": strconv.Itoa(targetPort)},
		})

		if err != nil {
			return fmt.Errorf("failed to communicate with daemon: %w", err)
		}

		if !resp.Success {
			return fmt.Errorf("failed to start tunnel: %s", resp.Error)
		}

		var url string
		json.Unmarshal(resp.Data, &url)

		fmt.Printf("✅ Tunnel active! Public URL: %s%s%s%s\n", Bold, Green, url, Reset)
		
		if err := clipboard.WriteAll(url); err == nil {
			fmt.Println("📋 URL copied to clipboard.")
		} else {
			fmt.Println("⚠️ Failed to write to clipboard, but your tunnel is running.")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(shareCmd)
}
