package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/N3M1K/xrp/internal/proxy"
	"github.com/N3M1K/xrp/internal/scanner"
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

func main() {
	processes, err := scanner.ScanProcesses()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s%sError scanning processes: %v%s\n", Bold, Red, err, Reset)
		os.Exit(1)
	}

	if len(processes) == 0 {
		fmt.Printf("%s%sNo relevant local services found running.%s\n", Bold, Yellow, Reset)
		return
	}

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

	// Generate and apply Caddy config
	err = proxy.StartCaddy()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to start Caddy stub: %v\n", err)
	}

	config := proxy.GenerateConfig(processes)
	err = proxy.ApplyConfig(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s%sWarning: Failed to apply Caddy config. Is Caddy running on :2019?%s\nError: %v\n\n", Bold, Yellow, Reset, err)
	} else {
		fmt.Printf("%s%sSuccess: Proxy configuration applied!%s\n\n", Bold, Green, Reset)
	}

	// Print Dashboard
	fmt.Printf("%s%s 🚀 XRP Dashboard %s\n", Bold, Cyan, Reset)
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
	for _, p := range processes {
		var url string
		if p.ProjectName != "" {
			url = fmt.Sprintf("http://%s.local", p.ProjectName)
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
