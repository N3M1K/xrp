//go:build windows

package scanner

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type WindowsScanner struct{}

func (s *WindowsScanner) Scan() ([]Process, error) {
	// Step 1: netstat -ano → port + PID
	portToPID, err := getListeningPorts()
	if err != nil {
		return nil, fmt.Errorf("netstat failed: %w", err)
	}

	if len(portToPID) == 0 {
		return nil, nil
	}

	// Step 2: tasklist → PID + process name
	pidToName, err := getProcessNames()
	if err != nil {
		return nil, fmt.Errorf("tasklist failed: %w", err)
	}

	// Step 3: WMIC → PID + executable path (for CWD)
	pidToCWD, err := getProcessCWDs()
	if err != nil {
		// Non-fatal — CWD is optional
		pidToCWD = make(map[int]string)
	}

	// Filtruj systémové procesy (vytaženo z loopu)
	systemProcesses := map[string]bool{
		"svchost": true, "lsass": true, "wininit": true,
		"spoolsv": true, "services": true, "system": true,
		"smss": true, "csrss": true, "winlogon": true,
	}

	// Filtruj specifický dev-noise background balast
	skipProcesses := map[string]bool{
		"antigravity":                 true,
		"spotify":                     true,
		"adobecollabsync":             true,
		"tailscaled":                  true,
		"riotclientservices":          true,
		"language_server_windows_x64": true,
		"plexscripthost":              true,
		"plextunerservice":            true,
		"joplin":                      true,
	}

	var processes []Process
	for port, pid := range portToPID {
		name := pidToName[pid]
		cwd := pidToCWD[pid]

		nameLower := strings.ToLower(name)

		if systemProcesses[nameLower] {
			continue
		}

		// Filtruj Windows ephemeral port range (49152-65535) pokud neni known
		if port >= 49152 && GetKnownApp(port) == "" {
			continue
		}

		if skipProcesses[nameLower] {
			if GetKnownApp(port) == "" {
				continue
			}
		}

		// If WMIC gave us a full exe path, use its directory as CWD
		if cwd != "" && !isDirectory(cwd) {
			cwd = filepath.Dir(cwd)
		}

		projectName := filepath.Base(cwd)
		if projectName == "" || projectName == "." || projectName == "\\" {
			// fallback na process name
			projectName = name
		}

		// URL Safe Slugifier (e.g. 'Plex Media Server' -> 'plex-media-server')
		projectName = strings.ToLower(strings.ReplaceAll(projectName, " ", "-"))

		knownApp := GetKnownApp(port)

		processes = append(processes, Process{
			PID:         pid,
			Port:        port,
			ProcessName: name,
			ProjectName: projectName,
			CWD:         cwd,
			KnownApp:    knownApp,
		})
	}

	return processes, nil
}

// getListeningPorts parses `netstat -ano` output.
// Returns map of port → PID for LISTENING TCP ports only.
func getListeningPorts() (map[int]int, error) {
	out, err := exec.Command("netstat", "-ano").Output()
	if err != nil {
		return nil, err
	}

	portToPID := make(map[int]int)
	lines := strings.Split(string(out), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Only TCP LISTENING lines
		if !strings.HasPrefix(line, "TCP") {
			continue
		}
		if !strings.Contains(line, "LISTENING") {
			continue
		}

		// Format: TCP  0.0.0.0:3000  0.0.0.0:0  LISTENING  1234
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		localAddr := fields[1] // e.g. "0.0.0.0:3000" or "[::]:8096"
		pidStr := fields[4]

		// Extract port from local address
		port, err := extractPort(localAddr)
		if err != nil {
			continue
		}

		pid, err := strconv.Atoi(strings.TrimSpace(pidStr))
		if err != nil {
			continue
		}

		portToPID[port] = pid
	}

	return portToPID, nil
}

// extractPort handles both IPv4 (0.0.0.0:3000) and IPv6 ([::]:8096) formats.
func extractPort(addr string) (int, error) {
	// Find last colon — works for both IPv4 and IPv6
	idx := strings.LastIndex(addr, ":")
	if idx == -1 {
		return 0, fmt.Errorf("no colon in address: %s", addr)
	}
	portStr := addr[idx+1:]
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid port: %s", portStr)
	}
	return port, nil
}

// getProcessNames parses `tasklist /fo csv /nh` output.
// Returns map of PID → process name.
func getProcessNames() (map[int]string, error) {
	out, err := exec.Command("tasklist", "/fo", "csv", "/nh").Output()
	if err != nil {
		return nil, err
	}

	pidToName := make(map[int]string)
	lines := strings.Split(string(out), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// CSV format: "chrome.exe","1234","Console","1","50,000 K"
		parts := parseCSVLine(line)
		if len(parts) < 2 {
			continue
		}

		name := strings.Trim(parts[0], "\"")
		pidStr := strings.Trim(parts[1], "\"")
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		// Strip .exe suffix for cleaner display
		name = strings.TrimSuffix(name, ".exe")
		pidToName[pid] = name
	}

	return pidToName, nil
}

// getProcessCWDs uses WMIC to get executable paths.
// Returns map of PID → executable path (not CWD, but best we can do on Windows).
func getProcessCWDs() (map[int]string, error) {
	out, err := exec.Command("wmic", "process", "get", "ProcessId,ExecutablePath", "/format:csv").Output()
	if err != nil {
		// WMIC might not be available on newer Windows — non-fatal
		return make(map[int]string), nil
	}

	pidToCWD := make(map[int]string)
	lines := strings.Split(string(out), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Node") {
			continue
		}

		// CSV: Node,ExecutablePath,ProcessId
		parts := strings.Split(line, ",")
		if len(parts) < 3 {
			continue
		}

		exePath := strings.TrimSpace(parts[1])
		pidStr := strings.TrimSpace(parts[2])

		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		if exePath != "" {
			pidToCWD[pid] = exePath
		}
	}

	return pidToCWD, nil
}

// parseCSVLine splits a CSV line respecting quoted fields.
func parseCSVLine(line string) []string {
	var fields []string
	var current strings.Builder
	inQuote := false

	for _, ch := range line {
		switch {
		case ch == '"':
			inQuote = !inQuote
			current.WriteRune(ch)
		case ch == ',' && !inQuote:
			fields = append(fields, current.String())
			current.Reset()
		default:
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		fields = append(fields, current.String())
	}
	return fields
}

// isDirectory checks if a path looks like a directory (not a file).
func isDirectory(path string) bool {
	return !strings.Contains(filepath.Base(path), ".")
}

func NewScanner() Scanner {
	return &WindowsScanner{}
}