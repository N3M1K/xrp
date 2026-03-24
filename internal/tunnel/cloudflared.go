package tunnel

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"sync"
)

var (
	tunnels    = make(map[string]*exec.Cmd)
	tunnelURLs = make(map[string]string)
	mu         sync.RWMutex
)

// CheckCloudflared checks if cloudflared is installed
func CheckCloudflared() error {
	_, err := exec.LookPath("cloudflared")
	if err != nil {
		return fmt.Errorf("cloudflared not found in PATH")
	}
	return nil
}

// StartTunnel launches a cloudflared proxy for the given port and maps it to the projectName
func StartTunnel(port int, projectName string) (string, error) {
	if err := CheckCloudflared(); err != nil {
		return "", err
	}

	mu.Lock()
	if _, exists := tunnels[projectName]; exists {
		mu.Unlock()
		return "", fmt.Errorf("tunnel for %s is already running", projectName)
	}
	mu.Unlock()

	cmd := exec.Command("cloudflared", "tunnel", "--url", fmt.Sprintf("http://localhost:%d", port))
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	if err := cmd.Start(); err != nil {
		return "", err
	}

	// Read stderr to find the URL
	urlChan := make(chan string)
	go func() {
		scanner := bufio.NewScanner(stderr)
		urlRegex := regexp.MustCompile(`https://[a-zA-Z0-9-]+\.trycloudflare\.com`)
		for scanner.Scan() {
			line := scanner.Text()
			matches := urlRegex.FindStringSubmatch(line)
			if len(matches) > 0 {
				urlChan <- matches[0]
				return
			}
		}
		close(urlChan)
	}()

	url := <-urlChan
	if url == "" {
		cmd.Process.Kill()
		return "", fmt.Errorf("failed to extract tunnel URL")
	}

	mu.Lock()
	tunnels[projectName] = cmd
	tunnelURLs[projectName] = url
	mu.Unlock()

	return url, nil
}

// StopTunnel kills the running cloudflared process for a project
func StopTunnel(projectName string) error {
	mu.Lock()
	defer mu.Unlock()

	cmd, exists := tunnels[projectName]
	if !exists {
		return fmt.Errorf("no tunnel running for %s", projectName)
	}

	cmd.Process.Kill()
	cmd.Process.Wait()
	delete(tunnels, projectName)
	delete(tunnelURLs, projectName)
	return nil
}

// StopAll stops all active tunnels (useful on daemon shutdown)
func StopAll() {
	mu.Lock()
	defer mu.Unlock()
	for proj, cmd := range tunnels {
		cmd.Process.Kill()
		cmd.Process.Wait()
		delete(tunnels, proj)
		delete(tunnelURLs, proj)
	}
}

// GetActiveTunnels returns a map of project names to tunnel URLs
func GetActiveTunnels() map[string]string {
	mu.RLock()
	defer mu.RUnlock()
	res := make(map[string]string)
	for k, v := range tunnelURLs {
		res[k] = v
	}
	return res
}
