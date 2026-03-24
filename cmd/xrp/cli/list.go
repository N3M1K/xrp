package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/N3M1K/xrp/internal/scanner"
	"github.com/N3M1K/xrp/internal/socket"
	"github.com/spf13/cobra"
)

// ANSI Color definitions
const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Purple = "\033[35m"
	Cyan   = "\033[36m"
	White  = "\033[37m"
	Bold   = "\033[1m"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all detected local services",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := socket.Send(socket.Request{Cmd: "list"})
		if err != nil {
			fmt.Printf("%s%sXRP daemon is not running. Start it with 'xrp start'.%s\n", Bold, Red, Reset)
			return nil
		}

		if !resp.Success {
			fmt.Printf("%s%sError: %s%s\n", Bold, Red, resp.Error, Reset)
			return nil
		}

		var processes []scanner.Process
		if err := json.Unmarshal(resp.Data, &processes); err != nil {
			return fmt.Errorf("failed to unmarshal process list: %w", err)
		}

		if len(processes) == 0 {
			fmt.Printf("%s%sNo relevant local services found running.%s\n", Bold, Yellow, Reset)
			return nil
		}

		printDashboard(processes)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func printDashboard(processes []scanner.Process) {
	// Calculate column widths
	portWidth := 6
	pidWidth := 7
	processWidth := 15
	projectWidth := 20
	appWidth := 15

	for _, p := range processes {
		if len(fmt.Sprintf("%d", p.Port)) > portWidth {
			portWidth = len(fmt.Sprintf("%d", p.Port))
		}
		if len(fmt.Sprintf("%d", p.PID)) > pidWidth {
			pidWidth = len(fmt.Sprintf("%d", p.PID))
		}
		if len(p.ProcessName) > processWidth {
			processWidth = len(p.ProcessName)
		}
		if len(p.ProjectName) > projectWidth {
			projectWidth = len(p.ProjectName)
		}
		if len(p.KnownApp) > appWidth {
			appWidth = len(p.KnownApp)
		}
	}

	// Print Dashboard
	fmt.Printf("\n%s%s 🚀 XRP Dashboard %s\n", Bold, Cyan, Reset)
	fmt.Println(strings.Repeat("=", portWidth+pidWidth+processWidth+projectWidth+appWidth+14))

	headerFormat := fmt.Sprintf("%s%%-%ds | %%-%ds | %%-%ds | %%-%ds | %%-%ds%s\n", Bold, portWidth, pidWidth, processWidth, projectWidth, appWidth, Reset)
	fmt.Printf(headerFormat, "PORT", "PID", "PROCESS", "PROJECT", "APP")
	fmt.Println(strings.Repeat("-", portWidth+pidWidth+processWidth+projectWidth+appWidth+14))

	for _, p := range processes {
		paddedPort := padRight(fmt.Sprintf("%d", p.Port), portWidth)
		paddedPid := padRight(fmt.Sprintf("%d", p.PID), pidWidth)
		paddedProcess := padRight(p.ProcessName, processWidth)
		paddedProject := padRight(p.ProjectName, projectWidth)
		paddedApp := padRight(p.KnownApp, appWidth)
		
		if p.ProjectName == "" {
			paddedProject = padRight("-", projectWidth)
		}
		if p.KnownApp == "" {
			paddedApp = padRight("-", appWidth)
		}

		fmt.Printf("%s%s%s | %s | %s%s%s | %s%s%s | %s%s%s\n", 
			Cyan, paddedPort, Reset, 
			paddedPid, 
			Yellow, paddedProcess, Reset, 
			Green, paddedProject, Reset, 
			Purple, paddedApp, Reset)
	}
	fmt.Println(strings.Repeat("=", portWidth+pidWidth+processWidth+projectWidth+appWidth+14))
	
	// Print actionable URLs
	fmt.Println("\nAvailable URLs:")
	tld := ".local"
	if cfg != nil && cfg.TLD != "" {
		tld = cfg.TLD
	}

	cleanTld := strings.TrimPrefix(tld, ".")

	for _, p := range processes {
		var url string
		if p.ProjectName != "" {
			url = fmt.Sprintf("https://%s.%s", p.ProjectName, cleanTld)
		} else {
			url = fmt.Sprintf("http://localhost:%d", p.Port)
		}
		fmt.Printf("➜ %s%s%s (Port %d)\n", Bold, url, Reset, p.Port)
	}
	fmt.Println()
}

func padRight(str string, length int) string {
	if len(str) >= length {
		return str
	}
	return str + strings.Repeat(" ", length-len(str))
}
