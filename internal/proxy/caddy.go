package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"

	"github.com/N3M1K/xrp/internal/deps"
	"github.com/N3M1K/xrp/internal/scanner"
)

// CaddyConfig represents the root of the Caddy JSON structure
type CaddyConfig struct {
	Apps Apps `json:"apps"`
}

type Apps struct {
	HTTP HTTPApp `json:"http"`
	TLS  *TLSApp `json:"tls,omitempty"`
}

type TLSApp struct {
	Certificates Certificates `json:"certificates"`
}

type Certificates struct {
	LoadFiles []LoadFile `json:"load_files"`
}

type LoadFile struct {
	Certificate string `json:"certificate"`
	Key         string `json:"key"`
}

type HTTPApp struct {
	Servers map[string]Server `json:"servers"`
}

type Server struct {
	Listen []string `json:"listen"`
	Routes []Route  `json:"routes"`
}

type Route struct {
	Match  []Match  `json:"match,omitempty"`
	Handle []Handle `json:"handle"`
}

type Match struct {
	Host []string `json:"host,omitempty"`
}

type Handle struct {
	Handler   string     `json:"handler"`
	Upstreams []Upstream `json:"upstreams,omitempty"`
}

type Upstream struct {
	Dial string `json:"dial"`
}

// GenerateConfig creates a Caddy JSON configuration from a list of scanned processes.
func GenerateConfig(processes []scanner.Process, tld string, certFile string, keyFile string) CaddyConfig {
	routes := []Route{}

	cleanTld := tld
	if cleanTld[0] == '.' {
		cleanTld = cleanTld[1:]
	}

	for _, p := range processes {
		if p.ProjectName == "" {
			continue // Skip if no project name can be determined
		}

		// Clean the hostname (basic cleaning for MVP)
		host := fmt.Sprintf("%s.%s", p.ProjectName, cleanTld)

		route := Route{
			Match: []Match{
				{Host: []string{host}},
			},
			Handle: []Handle{
				{
					Handler: "reverse_proxy",
					Upstreams: []Upstream{
						{Dial: fmt.Sprintf("localhost:%d", p.Port)},
					},
				},
			},
		}

		routes = append(routes, route)
	}

	config := CaddyConfig{
		Apps: Apps{
			HTTP: HTTPApp{
				Servers: map[string]Server{
					"xrp_server": {
						Listen: []string{":80", ":443"}, // Listen on HTTP and HTTPS
						Routes: routes,
					},
				},
			},
		},
	}

	if certFile != "" && keyFile != "" {
		config.Apps.TLS = &TLSApp{
			Certificates: Certificates{
				LoadFiles: []LoadFile{
					{
						Certificate: certFile,
						Key:         keyFile,
					},
				},
			},
		}
	}

	return config
}

// ApplyConfig posts the CaddyConfig to the local Caddy Admin API.
func ApplyConfig(config CaddyConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, "http://localhost:2019/load", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post config to caddy (is it running on :2019?): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("caddy API returned status: %d", resp.StatusCode)
	}

	return nil
}

// RemoveRoute is a stub for granular route removal via DELETE if needed.
// For now, ApplyConfig overwrites the entire config.
func RemoveRoute(port int) error {
	// MVP: Not implemented as ApplyConfig rebuilds all routes.
	// Future: DELETE /config/apps/http/servers/xrp_server/routes/...
	return nil
}

// StartCaddy actively starts the Caddy process in the background.
func StartCaddy() error {
	_, err := exec.LookPath("caddy")
	if err != nil {
		if runtime.GOOS == "darwin" {
			return fmt.Errorf("caddy not found. On macOS, you can install it via Homebrew: brew install caddy")
		}
		return fmt.Errorf("caddy not found in PATH")
	}

	// Check if already running via Admin API
	resp, err := http.Get("http://localhost:2019/config/")
	if err == nil && resp.StatusCode == 200 {
		return nil // Already running
	}

	cmd := exec.Command("caddy", "start")
	if err := cmd.Run(); err != nil {
		caddyPath, _ := exec.LookPath("caddy")
		return deps.WrapCaddyError(caddyPath, fmt.Errorf("caddy start failed: %w", err))
	}
	return nil
}

// StopCaddy gracefully stops the Caddy process.
func StopCaddy() error {
	cmd := exec.Command("caddy", "stop")
	return cmd.Run()
}
