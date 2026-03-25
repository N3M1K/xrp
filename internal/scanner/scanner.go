package scanner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ScanProcesses uses the OS-specific scanner to find listening ports,
// then filters, enriches, and returns the list of Processes.
func ScanProcesses() ([]Process, error) {
	scanner := NewScanner()
	rawProcesses, err := scanner.Scan()
	if err != nil {
		return nil, err
	}

	var processes []Process

	for _, p := range rawProcesses {
		// Filter out ports < 1024
		if p.Port < 1024 {
			continue
		}

		// Check system services
		if isSystemServicePort(p.Port) {
			// unless they have Dev context (e.g., ran from a non-system dir)
			if !isDevContext(p.CWD) {
				continue
			}
		}

		// Enrich Project Name if not already set or refined
		enrichedName := getProjectName(p.CWD)
		if enrichedName != "" {
			p.ProjectName = enrichedName
		}

		// Ensure the project name is always safely slugified for Caddy host matching
		p.ProjectName = strings.ToLower(strings.ReplaceAll(p.ProjectName, " ", "-"))

		processes = append(processes, p)
	}

	// Always sort by Port descending for deterministic UI presentation
	sort.SliceStable(processes, func(i, j int) bool {
		return processes[i].Port > processes[j].Port
	})

	return processes, nil
}

func isSystemServicePort(port int) bool {
	systemPorts := map[int]bool{
		22:   true,
		80:   true,
		443:  true,
		3306: true,
		5432: true,
	}
	return systemPorts[port]
}

func isDevContext(cwd string) bool {
	// A simple heuristic: if it's running in typical system paths, it's not dev context.
	// If the user is running it from their home directory or workspace, it is probably dev context.
	if cwd == "" || cwd == "/" || strings.HasPrefix(cwd, "/var") || strings.HasPrefix(cwd, "/etc") || strings.HasPrefix(cwd, "/usr") {
		return false
	}
	return true
}

type packageJSON struct {
	Name string `json:"name"`
}

func getProjectName(cwd string) string {
	if cwd == "" {
		return ""
	}

	// 1. Check package.json
	pkgStr, err := os.ReadFile(filepath.Join(cwd, "package.json"))
	if err == nil {
		var pkg packageJSON
		if err := json.Unmarshal(pkgStr, &pkg); err == nil && pkg.Name != "" {
			return pkg.Name
		}
	}

	// 2. Check Cargo.toml (simple string matching to avoid heavy TOML parser for MVP)
	cargoStr, err := os.ReadFile(filepath.Join(cwd, "Cargo.toml"))
	if err == nil {
		lines := strings.Split(string(cargoStr), "\n")
		inPackage := false
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "[package]" {
				inPackage = true
				continue
			}
			if inPackage && strings.HasPrefix(line, "[") {
				inPackage = false // entered another section
			}
			if inPackage && strings.HasPrefix(line, "name") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					name := strings.TrimSpace(parts[1])
					name = strings.Trim(name, `"'`)
					if name != "" {
						return name
					}
				}
			}
		}
	}

	// 3. Check pyproject.toml
	pyprojectStr, err := os.ReadFile(filepath.Join(cwd, "pyproject.toml"))
	if err == nil {
		lines := strings.Split(string(pyprojectStr), "\n")
		inProject := false
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "[project]" || line == "[tool.poetry]" {
				inProject = true
				continue
			}
			if inProject && strings.HasPrefix(line, "[") {
				inProject = false
			}
			if inProject && strings.HasPrefix(line, "name") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					name := strings.TrimSpace(parts[1])
					name = strings.Trim(name, `"'`)
					if name != "" {
						return name
					}
				}
			}
		}
	}

	// 4. Fallback to base dir name
	base := filepath.Base(cwd)
	if base == "." || base == "/" {
		return ""
	}
	return base
}
