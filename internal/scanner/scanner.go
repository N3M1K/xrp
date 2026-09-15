package scanner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ipcPort is the xrp daemon IPC port and must never be proxied.
const ipcPort = 40192

// systemProcesses are OS/background processes that never represent a dev server.
var systemProcesses = map[string]bool{
	// Windows
	"svchost": true, "lsass": true, "wininit": true, "spoolsv": true,
	"services": true, "system": true, "smss": true, "csrss": true, "winlogon": true,
	// xrp ecosystem (prevents recursive proxying)
	"xrp": true, "xrp.exe": true, "xrp-daemon": true, "xrp-daemon.exe": true,
	"caddy": true, "caddy.exe": true,
}

// noiseProcesses are common desktop apps that hold listening sockets but are
// not development servers. They are only skipped when the port is not a
// well-known dev/service port.
var noiseProcesses = map[string]bool{
	"spotify": true, "onedrive": true, "dropbox": true, "discord": true,
	"slack": true, "teams": true, "ms-teams": true, "antigravity": true,
	"adobecollabsync": true, "riotclientservices": true, "joplin": true,
	"plexscripthost": true, "plextunerservice": true,
	"language_server_windows_x64": true,
}

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
		// Filter out privileged and IPC ports.
		if p.Port < 1024 || p.Port == ipcPort {
			continue
		}

		nameLower := strings.ToLower(p.ProcessName)
		if systemProcesses[nameLower] {
			continue
		}

		// Noise reduction: skip desktop apps unless they own a known port.
		if noiseProcesses[nameLower] && GetKnownApp(p.Port) == "" {
			continue
		}

		// Ephemeral port range: only interesting when the port is a known one.
		if p.Port >= 49152 && GetKnownApp(p.Port) == "" {
			continue
		}

		// Well-known system service ports need genuine dev context to qualify.
		if isSystemServicePort(p.Port) && !isDevContext(p.CWD) {
			continue
		}

		// Enrich Project Name if not already set or refined.
		if enrichedName := getProjectName(p.CWD); enrichedName != "" {
			p.ProjectName = enrichedName
		}

		p.ProjectName = Slugify(p.ProjectName)
		if p.ProjectName == "" {
			p.ProjectName = Slugify(p.ProcessName)
		}
		if p.ProjectName == "" {
			continue
		}

		if p.Addr == "" {
			p.Addr = "127.0.0.1"
		}

		processes = append(processes, p)
	}

	// Deterministic ordering (by port, then name) for stable UI/config output.
	sort.SliceStable(processes, func(i, j int) bool {
		if processes[i].Port != processes[j].Port {
			return processes[i].Port < processes[j].Port
		}
		return processes[i].ProjectName < processes[j].ProjectName
	})

	return processes, nil
}

// normalizeDialAddr turns a listen address host part into a dialable loopback host.
func normalizeDialAddr(host string) string {
	host = strings.Trim(host, "[]")
	switch host {
	case "", "*", "0.0.0.0", "::", "localhost":
		return "127.0.0.1"
	}
	return host
}

// splitHostPort is a tolerant variant of net.SplitHostPort that also accepts
// values without a port and IPv6 literals without brackets.
func splitHostPort(value string) (host string, port string, ok bool) {
	if strings.HasPrefix(value, "[") {
		end := strings.LastIndex(value, "]")
		if end == -1 {
			return "", "", false
		}
		host = value[1:end]
		if idx := strings.LastIndex(value, ":"); idx > end {
			port = value[idx+1:]
		}
		return host, port, port != ""
	}
	idx := strings.LastIndex(value, ":")
	if idx == -1 {
		return "", "", false
	}
	return value[:idx], value[idx+1:], true
}

// Slugify converts an arbitrary project name into a safe DNS label.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimLeft(s, ". ")

	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || r == ' ' || r == '.':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
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
	if cwd == "" || cwd == "/" || cwd == string(filepath.Separator) {
		return false
	}
	systemPrefixes := []string{"/var", "/etc", "/usr", "/bin", "/sbin", "/lib", "/opt", "/run", "/proc", "/sys"}
	for _, prefix := range systemPrefixes {
		if cwd == prefix || strings.HasPrefix(cwd, prefix+"/") {
			return false
		}
	}
	return true
}

type packageJSON struct {
	Name string `json:"name"`
}

func getProjectName(cwd string) string {
	if cwd == "" || cwd == "/" || cwd == string(filepath.Separator) {
		return ""
	}

	// 1. Check package.json
	if pkgStr, err := os.ReadFile(filepath.Join(cwd, "package.json")); err == nil {
		var pkg packageJSON
		if err := json.Unmarshal(pkgStr, &pkg); err == nil && pkg.Name != "" {
			return pkg.Name
		}
	}

	// 2. Check Cargo.toml
	if cargoStr, err := os.ReadFile(filepath.Join(cwd, "Cargo.toml")); err == nil {
		if name := tomlSectionName(string(cargoStr), "[package]"); name != "" {
			return name
		}
	}

	// 3. Check pyproject.toml
	if pyStr, err := os.ReadFile(filepath.Join(cwd, "pyproject.toml")); err == nil {
		if name := tomlSectionName(string(pyStr), "[project]"); name != "" {
			return name
		}
		if name := tomlSectionName(string(pyStr), "[tool.poetry]"); name != "" {
			return name
		}
	}

	// 4. Fallback to base dir name
	base := filepath.Base(cwd)
	if base == "." || base == "/" || base == string(filepath.Separator) || strings.HasPrefix(base, ".") {
		return ""
	}

	// 5. If the base dir is a common output directory (bin, build, dist…) use
	//    the parent directory instead — e.g. ./build/bin -> project name.
	commonOutDirs := map[string]bool{
		"bin": true, "build": true, "dist": true, "out": true,
		"target": true, "release": true, "debug": true,
	}
	if commonOutDirs[strings.ToLower(base)] {
		parent := filepath.Dir(cwd)
		parentBase := filepath.Base(parent)
		if parent != "" && parent != "." && parent != "/" &&
			parent != string(filepath.Separator) && parent != cwd &&
			parentBase != "." && parentBase != "/" && parentBase != string(filepath.Separator) {
			return parentBase
		}
	}

	return base
}

// tomlSectionName extracts the `name = "..."` value from a TOML section header.
func tomlSectionName(content, section string) string {
	lines := strings.Split(content, "\n")
	inSection := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == section {
			inSection = true
			continue
		}
		if inSection && strings.HasPrefix(line, "[") {
			inSection = false
			continue
		}
		if inSection && strings.HasPrefix(line, "name") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			name := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			if name != "" {
				return name
			}
		}
	}
	return ""
}
