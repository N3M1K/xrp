//go:build darwin

package scanner

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type DarwinScanner struct{}

func (s *DarwinScanner) Scan() ([]Process, error) {
	// lsof -iTCP -sTCP:LISTEN -Fpn
	cmd := exec.Command("lsof", "-iTCP", "-sTCP:LISTEN", "-Fpn")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var processes []Process
	lines := strings.Split(string(out), "\n")
	
	currentPid := -1

	for _, line := range lines {
		if line == "" {
			continue
		}
		
		fieldType := line[0]
		value := line[1:]

		switch fieldType {
		case 'p':
			pid, err := strconv.Atoi(value)
			if err == nil {
				currentPid = pid
			}
		case 'n':
			// value is like "*:8080" or "localhost:3000" or "127.0.0.1:4000"
			parts := strings.Split(value, ":")
			if len(parts) >= 2 {
				portStr := parts[len(parts)-1]
				port, err := strconv.Atoi(portStr)
				if err == nil && currentPid != -1 {
					procName := getProcessName(currentPid)
					cwd := getProcessCwd(currentPid)
					
					// projectName will be refined in the main scanner logic, but we can do a fallback here
					projectName := filepath.Base(cwd)
					if projectName == "." || projectName == "/" {
						projectName = ""
					}
					
					knownApp := GetKnownApp(port)
					
					processes = append(processes, Process{
						PID:         currentPid,
						Port:        port,
						ProcessName: procName,
						ProjectName: projectName,
						CWD:         cwd,
						KnownApp:    knownApp,
					})
				}
			}
		}
	}

	return processes, nil
}

func getProcessName(pid int) string {
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func getProcessCwd(pid int) string {
	// lsof -a -p PID -d cwd -Fn
	cmd := exec.Command("lsof", "-a", "-p", strconv.Itoa(pid), "-d", "cwd", "-Fn")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "n") {
			return line[1:]
		}
	}
	return ""
}

func NewScanner() Scanner {
	return &DarwinScanner{}
}
