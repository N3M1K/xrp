package hosts

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
)

const marker = "# xrp-managed"

var mu sync.Mutex

func getHostsPath() string {
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
// Removes stale xrp-managed entries that are no longer active.
func SyncEntries(hostnames []string) error {
	mu.Lock()
	defer mu.Unlock()

	path := getHostsPath()

	// Read current file
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read hosts file: %w", err)
	}

	lines := strings.Split(string(data), "\n")

	// Build a set of desired hostnames
	desired := make(map[string]bool)
	for _, h := range hostnames {
		desired[strings.ToLower(h)] = true
	}

	// Filter out old xrp-managed lines
	var kept []string
	for _, line := range lines {
		if strings.Contains(line, marker) {
			continue // remove old xrp entries
		}
		kept = append(kept, line)
	}

	// Add new entries
	for _, h := range hostnames {
		entry := fmt.Sprintf("127.0.0.1 %s %s", h, marker)
		kept = append(kept, entry)
	}

	// Ensure file ends with newline
	output := strings.Join(kept, "\n")
	if !strings.HasSuffix(output, "\n") {
		output += "\n"
	}

	if err := os.WriteFile(path, []byte(output), 0644); err != nil {
		return fmt.Errorf("cannot write hosts file (are you running as admin?): %w", err)
	}

	return nil
}

// RemoveAllEntries removes all xrp-managed entries from the hosts file.
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
