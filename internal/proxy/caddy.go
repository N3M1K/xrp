package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/N3M1K/xrp/internal/config"
	"github.com/N3M1K/xrp/internal/deps"
	"github.com/N3M1K/xrp/internal/scanner"
	"github.com/N3M1K/xrp/internal/ssl"
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
	Listen                []string              `json:"listen"`
	Routes                []Route               `json:"routes"`
	TLSConnectionPolicies []TLSConnectionPolicy `json:"tls_connection_policies,omitempty"`
	AutoHTTPS             *AutoHTTPSConfig      `json:"automatic_https,omitempty"`
}

// TLSConnectionPolicy instructs Caddy which TLS config to use for incoming connections.
// An empty policy (no fields set) matches all connections and uses any loaded certificate.
type TLSConnectionPolicy struct{}

// AutoHTTPSConfig controls Caddy's automatic HTTPS / ACME behaviour.
type AutoHTTPSConfig struct {
	Disable bool `json:"disable"`
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
func GenerateConfig(processes []scanner.Process, cfg *config.Config, certPairs []ssl.CertPair) CaddyConfig {
	routes := []Route{}

	for _, p := range processes {
		if p.ProjectName == "" {
			continue // Skip if no project name can be determined
		}

		tld := cfg.TLD
		if customTld, ok := cfg.ProjectTLDs[p.ProjectName]; ok && customTld != "" {
			tld = customTld
		}
		cleanTld := strings.TrimPrefix(tld, ".")

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

	// Always disable Caddy's automatic HTTPS/ACME — we manage certs via mkcert.
	// tls_connection_policies with an empty policy tells Caddy to use any
	// locally-loaded certificate that matches the incoming SNI hostname.
	// This is the correct approach for local dev / home lab: zero config for the user.
	xrpServer := Server{
		Listen: []string{
			fmt.Sprintf(":%d", cfg.HTTPPort),
			fmt.Sprintf(":%d", cfg.HTTPSPort),
		},
		Routes:    routes,
		AutoHTTPS: &AutoHTTPSConfig{Disable: true},
	}

	if len(certPairs) > 0 {
		// An empty TLSConnectionPolicy matches every connection and instructs
		// Caddy to pick the best-matching cert from the load_files pool.
		xrpServer.TLSConnectionPolicies = []TLSConnectionPolicy{{}}
	}

	config := CaddyConfig{
		Apps: Apps{
			HTTP: HTTPApp{
				Servers: map[string]Server{
					"xrp_server": xrpServer,
				},
			},
		},
	}

	if len(certPairs) > 0 {
		var loadFiles []LoadFile
		for _, pair := range certPairs {
			if pair.Cert != "" && pair.Key != "" {
				loadFiles = append(loadFiles, LoadFile{
					Certificate: pair.Cert,
					Key:         pair.Key,
				})
			}
		}
		if len(loadFiles) > 0 {
			config.Apps.TLS = &TLSApp{
				Certificates: Certificates{
					LoadFiles: loadFiles,
				},
			}
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

// resolveCaddyBinary finds the caddy binary either from PATH or from the deps cache.
func resolveCaddyBinary() (string, error) {
	// 1. Try PATH first
	if sysPath, err := exec.LookPath("caddy"); err == nil {
		return sysPath, nil
	}

	// 2. Try the deps cache directory
	binName := "caddy"
	if runtime.GOOS == "windows" {
		binName = "caddy.exe"
	}

	cacheDir, err := deps.GetBinDir()
	if err != nil {
		return "", fmt.Errorf("caddy not found in PATH and cannot locate deps cache: %w", err)
	}

	cachePath := filepath.Join(cacheDir, binName)
	if stat, err := os.Stat(cachePath); err == nil && !stat.IsDir() && stat.Size() > 0 {
		return cachePath, nil
	}

	return "", fmt.Errorf("caddy not found in PATH or deps cache (%s)", cacheDir)
}

// StartCaddy actively starts the Caddy process in the background.
func StartCaddy() error {
	caddyPath, err := resolveCaddyBinary()
	if err != nil {
		if runtime.GOOS == "darwin" {
			return fmt.Errorf("caddy not found. On macOS, you can install it via Homebrew: brew install caddy")
		}
		return err
	}

	// Check if already running via Admin API
	resp, err := http.Get("http://localhost:2019/config/")
	if err == nil && resp.StatusCode == 200 {
		return nil // Already running
	}

	cmd := exec.Command(caddyPath, "start")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		return deps.WrapCaddyError(caddyPath, fmt.Errorf("caddy start failed: %s", errMsg))
	}
	return nil
}

// StopCaddy gracefully stops the Caddy process.
func StopCaddy() error {
	caddyPath, err := resolveCaddyBinary()
	if err != nil {
		return nil // If we can't find caddy, nothing to stop
	}
	cmd := exec.Command(caddyPath, "stop")
	return cmd.Run()
}
