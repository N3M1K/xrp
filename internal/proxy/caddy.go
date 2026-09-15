package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/N3M1K/halo-proxy/internal/config"
	"github.com/N3M1K/halo-proxy/internal/deps"
	"github.com/N3M1K/halo-proxy/internal/scanner"
	"github.com/N3M1K/halo-proxy/internal/ssl"
)

// CaddyConfig represents the root of the Caddy JSON structure
type CaddyConfig struct {
	Admin *AdminConfig `json:"admin,omitempty"`
	Apps  Apps         `json:"apps"`
}

// AdminConfig pins the Caddy admin API to the configured loopback port.
type AdminConfig struct {
	Listen string `json:"listen,omitempty"`
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
	Handler    string              `json:"handler"`
	Upstreams  []Upstream          `json:"upstreams,omitempty"`
	StatusCode int                 `json:"status_code,omitempty"`
	Headers    map[string][]string `json:"headers,omitempty"`
}

type Upstream struct {
	Dial string `json:"dial"`
}

// GenerateConfig creates a Caddy JSON configuration from a list of scanned processes.
//
// It builds two dedicated servers so plain HTTP and HTTPS behave correctly:
//   - halo_http  : redirects everything to HTTPS (308)
//   - halo_https : terminates TLS with the mkcert certs and reverse-proxies routes
//
// If no certificates are available it degrades gracefully to a plain HTTP proxy.
func GenerateConfig(processes []scanner.Process, cfg *config.Config, certPairs []ssl.CertPair) CaddyConfig {
	var routes []Route
	seenHost := make(map[string]bool)

	for _, p := range processes {
		if p.ProjectName == "" {
			continue
		}

		host := fmt.Sprintf("%s.%s", p.ProjectName, cfg.EffectiveTLD(p.ProjectName))
		if seenHost[host] {
			continue // one route per hostname; first (lowest) port wins
		}
		seenHost[host] = true

		dialHost := p.Addr
		if dialHost == "" {
			dialHost = "127.0.0.1"
		}

		routes = append(routes, Route{
			Match: []Match{{Host: []string{host}}},
			Handle: []Handle{{
				Handler:   "reverse_proxy",
				Upstreams: []Upstream{{Dial: net.JoinHostPort(dialHost, strconv.Itoa(p.Port))}},
			}},
		})
	}

	servers := make(map[string]Server)
	httpsEnabled := len(certPairs) > 0 && hasUsableCerts(certPairs)

	if httpsEnabled {
		httpsServer := Server{
			Listen:                []string{fmt.Sprintf(":%d", cfg.HTTPSPort)},
			Routes:                routes,
			TLSConnectionPolicies: []TLSConnectionPolicy{{}},
			AutoHTTPS:             &AutoHTTPSConfig{Disable: true},
		}
		servers["halo_https"] = httpsServer

		// Plain HTTP simply redirects to the matching HTTPS URL.
		if cfg.HTTPPort != cfg.HTTPSPort {
			servers["halo_http"] = Server{
				Listen:    []string{fmt.Sprintf(":%d", cfg.HTTPPort)},
				AutoHTTPS: &AutoHTTPSConfig{Disable: true},
				Routes: []Route{{
					Handle: []Handle{{
						Handler:    "static_response",
						StatusCode: http.StatusPermanentRedirect,
						Headers: map[string][]string{
							"Location": {"https://{http.request.host}{http.request.uri}"},
						},
					}},
				}},
			}
		}
	} else {
		// No trusted certs: serve the routes over plain HTTP so the user still
		// gets a working proxy instead of a hard failure.
		servers["halo_http"] = Server{
			Listen:    []string{fmt.Sprintf(":%d", cfg.HTTPPort)},
			Routes:    routes,
			AutoHTTPS: &AutoHTTPSConfig{Disable: true},
		}
	}

	c := CaddyConfig{
		Admin: &AdminConfig{Listen: fmt.Sprintf("127.0.0.1:%d", cfg.CaddyPort)},
		Apps: Apps{
			HTTP: HTTPApp{Servers: servers},
		},
	}

	if httpsEnabled {
		var loadFiles []LoadFile
		for _, pair := range certPairs {
			if pair.Cert != "" && pair.Key != "" {
				loadFiles = append(loadFiles, LoadFile{Certificate: pair.Cert, Key: pair.Key})
			}
		}
		if len(loadFiles) > 0 {
			c.Apps.TLS = &TLSApp{Certificates: Certificates{LoadFiles: loadFiles}}
		}
	}

	return c
}

func hasUsableCerts(pairs []ssl.CertPair) bool {
	for _, p := range pairs {
		if p.Cert != "" && p.Key != "" {
			return true
		}
	}
	return false
}

func adminURL(cfg *config.Config, path string) string {
	return fmt.Sprintf("http://127.0.0.1:%d%s", cfg.CaddyPort, path)
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

// ApplyConfig posts the CaddyConfig to the Caddy Admin API.
func ApplyConfig(cfg *config.Config, caddyConfig CaddyConfig) error {
	data, err := json.Marshal(caddyConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, adminURL(cfg, "/load"), bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post config to caddy (is it running on :%d?): %w", cfg.CaddyPort, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		msg := strings.TrimSpace(string(body))
		lower := strings.ToLower(msg)
		if strings.Contains(lower, "permission denied") || strings.Contains(lower, "access is denied") {
			bin, _ := resolveCaddyBinary()
			if runtime.GOOS == "windows" {
				return fmt.Errorf("caddy could not bind privileged ports (80/443). Restart halo from an Administrator terminal.\n%s", msg)
			}
			return fmt.Errorf("caddy could not bind privileged ports (80/443). Run: sudo setcap cap_net_bind_service=+ep %s\n%s", bin, msg)
		}
		if strings.Contains(lower, "address already in use") || strings.Contains(lower, "only one usage of each socket") {
			return fmt.Errorf("caddy could not bind ports %d/%d — something else is already listening (another proxy or a root web server).\n%s", cfg.HTTPPort, cfg.HTTPSPort, msg)
		}
		return fmt.Errorf("caddy API returned status %d: %s", resp.StatusCode, msg)
	}

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

// StartCaddy starts the Caddy process in the background with a minimal
// bootstrap config that only enables the admin API. The real configuration is
// then pushed via ApplyConfig.
func StartCaddy(cfg *config.Config) error {
	caddyPath, err := resolveCaddyBinary()
	if err != nil {
		if runtime.GOOS == "darwin" {
			return fmt.Errorf("caddy not found. On macOS, you can install it via Homebrew: brew install caddy")
		}
		return err
	}

	// Already running? Then there is nothing to do.
	if resp, err := httpClient.Get(adminURL(cfg, "/config/")); err == nil {
		resp.Body.Close()
		if resp.StatusCode < 400 {
			return nil
		}
	}

	bootstrapPath, err := writeBootstrapConfig(cfg)
	if err != nil {
		return err
	}

	// NOTE: `caddy start` forks a background child that inherits our stdout/stderr.
	// If we used CombinedOutput/bytes.Buffer here, Go would wait for the copied
	// pipe to close and block until Caddy itself exits. Redirect to a real file
	// so cmd.Run() only waits for the `caddy start` supervisor to return.
	logPath := filepath.Join(filepath.Dir(bootstrapPath), "caddy-start.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer logFile.Close()

	cmd := exec.Command(caddyPath, "start", "--config", bootstrapPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Run(); err != nil {
		msg := readFileString(logPath)
		if msg == "" {
			msg = err.Error()
		}
		return deps.WrapCaddyError(caddyPath, fmt.Errorf("caddy start failed: %s", msg))
	}
	return nil
}

func readFileString(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// writeBootstrapConfig writes a minimal JSON config that pins the admin API to
// the configured loopback port.
func writeBootstrapConfig(cfg *config.Config) (string, error) {
	binDir, err := deps.GetBinDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", err
	}

	path := filepath.Join(binDir, "caddy-bootstrap.json")
	payload := fmt.Sprintf("{\"admin\":{\"listen\":\"127.0.0.1:%d\"}}", cfg.CaddyPort)
	if err := os.WriteFile(path, []byte(payload), 0644); err != nil {
		return "", err
	}
	return path, nil
}

// StopCaddy gracefully stops the Caddy process.
func StopCaddy(cfg *config.Config) error {
	caddyPath, err := resolveCaddyBinary()
	if err != nil {
		return nil // If we can't find caddy, nothing to stop
	}
	cmd := exec.Command(caddyPath, "stop", "--address", fmt.Sprintf("127.0.0.1:%d", cfg.CaddyPort))
	return cmd.Run()
}
