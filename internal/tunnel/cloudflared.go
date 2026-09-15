package tunnel

import (
	"bufio"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"time"
)

var (
	tunnels    = make(map[string]*exec.Cmd)
	tunnelURLs = make(map[string]string)
	mu         sync.RWMutex
)

var urlRegex = regexp.MustCompile(`https://[a-zA-Z0-9-]+\.trycloudflare\.com`)

// CheckCloudflared checks if cloudflared is installed
func CheckCloudflared() error {
	_, err := exec.LookPath("cloudflared")
	if err != nil {
		return fmt.Errorf("cloudflared not found in PATH")
	}
	return nil
}

// StartTunnel launches a cloudflared quick tunnel for the given local address
// and maps it to the project name.
func StartTunnel(host string, port int, projectName string) (string, error) {
	if err := CheckCloudflared(); err != nil {
		return "", err
	}

	mu.Lock()
	if _, exists := tunnels[projectName]; exists {
		mu.Unlock()
		return "", fmt.Errorf("tunnel for %s is already running", projectName)
	}
	mu.Unlock()

	if host == "" {
		host = "127.0.0.1"
	}
	target := "http://" + net.JoinHostPort(host, strconv.Itoa(port))

	cmd := exec.Command("cloudflared", "tunnel", "--url", target)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	if err := cmd.Start(); err != nil {
		return "", err
	}

	// Buffered so the reader goroutine never blocks and can keep draining
	// cloudflared's stderr (otherwise the pipe buffer could fill up).
	urlChan := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		sent := false
		for scanner.Scan() {
			if sent {
				continue
			}
			if match := urlRegex.FindString(scanner.Text()); match != "" {
				urlChan <- match
				sent = true
			}
		}
		if !sent {
			urlChan <- ""
		}
	}()

	select {
	case url := <-urlChan:
		if url == "" {
			kill(cmd)
			return "", fmt.Errorf("failed to extract tunnel URL (cloudflared exited early)")
		}
		mu.Lock()
		tunnels[projectName] = cmd
		tunnelURLs[projectName] = url
		mu.Unlock()
		return url, nil

	case <-time.After(30 * time.Second):
		kill(cmd)
		return "", fmt.Errorf("timed out waiting for cloudflared to publish a URL")
	}
}

// StopTunnel kills the running cloudflared process for a project
func StopTunnel(projectName string) error {
	mu.Lock()
	defer mu.Unlock()

	cmd, exists := tunnels[projectName]
	if !exists {
		return fmt.Errorf("no tunnel running for %s", projectName)
	}

	kill(cmd)
	delete(tunnels, projectName)
	delete(tunnelURLs, projectName)
	return nil
}

// StopAll stops all active tunnels (useful on daemon shutdown)
func StopAll() {
	mu.Lock()
	defer mu.Unlock()
	for proj, cmd := range tunnels {
		kill(cmd)
		delete(tunnels, proj)
		delete(tunnelURLs, proj)
	}
}

// GetActiveTunnels returns a map of project names to tunnel URLs
func GetActiveTunnels() map[string]string {
	mu.RLock()
	defer mu.RUnlock()
	res := make(map[string]string, len(tunnelURLs))
	for k, v := range tunnelURLs {
		res[k] = v
	}
	return res
}

// kill terminates and reaps a cloudflared process.
func kill(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
}
