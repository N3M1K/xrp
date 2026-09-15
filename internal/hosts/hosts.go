package hosts

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
)

const marker = "# halo-managed"

var mu sync.Mutex

func getHostsPath() string {
	if override := os.Getenv("HALO_HOSTS_PATH"); override != "" {
		return override
	}
	if runtime.GOOS == "windows" {
		return `C:\Windows\System32\drivers\etc\hosts`
	}
	return "/etc/hosts"
}

// IsWritable checks if the hosts file can actually be written to.
func IsWritable() bool {
	path := getHostsPath()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// SyncEntries ensures all given hostnames are present in the hosts file as 127.0.0.1 entries.
// Removes stale halo-managed entries that are no longer active.
func SyncEntries(hostnames []string) error {
	mu.Lock()
	defer mu.Unlock()

	path := getHostsPath()

	// Read current file
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read hosts file: %w", err)
	}

	// Normalise line endings: on Windows hosts file uses \r\n.
	// We strip \r universally so marker matching works cross-platform.
	normalised := strings.ReplaceAll(string(data), "\r\n", "\n")
	normalised = strings.ReplaceAll(normalised, "\r", "\n")
	lines := strings.Split(normalised, "\n")

	// Build an ordered, de-duplicated set of desired hostnames
	seen := make(map[string]bool)
	desired := make([]string, 0, len(hostnames))
	for _, h := range hostnames {
		h = strings.TrimSpace(strings.ToLower(h))
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		desired = append(desired, h)
	}

	// Filter out old halo-managed lines and empty trailing lines
	var kept []string
	for _, line := range lines {
		line = strings.TrimRight(line, "\r") // belt-and-suspenders trim
		if strings.Contains(line, marker) {
			continue // remove old halo entries
		}
		kept = append(kept, line)
	}

	// Remove trailing empty lines to avoid bloat
	for len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
		kept = kept[:len(kept)-1]
	}

	// Add new entries
	for _, h := range desired {
		entry := fmt.Sprintf("127.0.0.1 %s %s", h, marker)
		kept = append(kept, entry)
	}

	// Ensure file ends with a single newline
	output := strings.Join(kept, "\n") + "\n"

	if err := os.WriteFile(path, []byte(output), 0644); err != nil {
		return fmt.Errorf("cannot write hosts file (are you running as admin?): %w", err)
	}

	return nil
}

// RemoveAllEntries removes all halo-managed entries from the hosts file.
func RemoveAllEntries() error {
	mu.Lock()
	defer mu.Unlock()

	path := getHostsPath()

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read hosts file: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var kept []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, marker) {
			continue
		}
		kept = append(kept, line)
	}

	output := strings.Join(kept, "\n")
	if !strings.HasSuffix(output, "\n") {
		output += "\n"
	}

	return os.WriteFile(path, []byte(output), 0644)
}
