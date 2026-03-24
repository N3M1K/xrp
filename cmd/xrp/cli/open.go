package cli

import (
	"fmt"
	"strings"

	"github.com/N3M1K/xrp/internal/socket"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open [project]",
	Short: "Open a specific project URL in the default browser",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project := args[0]
		
		tld := ".local"
		if cfg != nil && cfg.TLD != "" {
			tld = cfg.TLD
		}
		cleanTld := strings.TrimPrefix(tld, ".")
		url := fmt.Sprintf("https://%s.%s", project, cleanTld)

		fmt.Printf("Opening %s...\n", url)
		
		resp, err := socket.Send(socket.Request{
			Cmd:  "open",
			Args: map[string]string{"url": url},
		})
		
		if err != nil {
			fmt.Printf("\033[31mXRP daemon is not running. Start it with 'xrp start'.\033[0m\n")
			return nil
		}

		if !resp.Success {
			return fmt.Errorf("failed: %s", resp.Error)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(openCmd)
}
