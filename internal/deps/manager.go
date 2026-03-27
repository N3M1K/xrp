package deps

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type ResolvedDeps struct {
	Caddy       string
	Mkcert      string
	Cloudflared string
}

// CaddyPermissionError is raised on Linux systems when the downloaded isolated binary
// natively lacks required capabilities to intercept low-tier web traffic like port 80/443.
type CaddyPermissionError struct {
	Path string
	Err  error
}

func (e *CaddyPermissionError) Error() string {
	return fmt.Sprintf("⚠️ Caddy needs permission to bind ports 80/443. Run:\nsudo setcap cap_net_bind_service=+ep %s\nOriginal error: %v", e.Path, e.Err)
}

func WrapCaddyError(path string, err error) error {
	if runtime.GOOS == "linux" && err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "permission denied") || strings.Contains(strings.ToLower(err.Error()), "bind: permission denied") {
			return &CaddyPermissionError{Path: path, Err: err}
		}
	}
	return err
}

// GetBinDir returns the path to the XRP local binary cache directory.
func GetBinDir() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "xrp", "bin"), nil
}

// Security checksum maps to aggressively combat supply chain risks.
// Provide SHA256 hashes corresponding strictly to mapped OS/ARCH binaries.
// Blank hashes skip explicit checks during development.
var caddyChecksums = map[string]map[string]string{
	"linux":   {"amd64": "", "arm64": ""},
	"darwin":  {"amd64": "", "arm64": ""},
	"windows": {"amd64": ""},
}

var mkcertChecksums = map[string]map[string]string{
	"linux":   {"amd64": "", "arm64": ""},
	"darwin":  {"amd64": "", "arm64": ""},
	"windows": {"amd64": ""},
}

var cloudflaredChecksums = map[string]map[string]string{
	"linux":   {"amd64": "", "arm64": ""},
	"darwin":  {"amd64": ""},
	"windows": {"amd64": ""},
}

const (
	caddyVersion       = "2.9.1"
	mkcertVersion      = "1.4.4"
	cloudflaredVersion = "2024.12.0"
)

var caddyURLs = map[string]map[string]string{
	"linux": {
		"amd64": "https://github.com/caddyserver/caddy/releases/download/v" + caddyVersion + "/caddy_" + caddyVersion + "_linux_amd64.tar.gz",
		"arm64": "https://github.com/caddyserver/caddy/releases/download/v" + caddyVersion + "/caddy_" + caddyVersion + "_linux_arm64.tar.gz",
	},
	"darwin": {
		"amd64": "https://github.com/caddyserver/caddy/releases/download/v" + caddyVersion + "/caddy_" + caddyVersion + "_mac_amd64.tar.gz",
		"arm64": "https://github.com/caddyserver/caddy/releases/download/v" + caddyVersion + "/caddy_" + caddyVersion + "_mac_arm64.tar.gz",
	},
	"windows": {
		"amd64": "https://github.com/caddyserver/caddy/releases/download/v" + caddyVersion + "/caddy_" + caddyVersion + "_windows_amd64.zip",
	},
}

var mkcertURLs = map[string]map[string]string{
	"linux": {
		"amd64": "https://github.com/FiloSottile/mkcert/releases/download/v" + mkcertVersion + "/mkcert-v" + mkcertVersion + "-linux-amd64",
		"arm64": "https://github.com/FiloSottile/mkcert/releases/download/v" + mkcertVersion + "/mkcert-v" + mkcertVersion + "-linux-arm64",
	},
	"darwin": {
		"amd64": "https://github.com/FiloSottile/mkcert/releases/download/v" + mkcertVersion + "/mkcert-v" + mkcertVersion + "-darwin-amd64",
		"arm64": "https://github.com/FiloSottile/mkcert/releases/download/v" + mkcertVersion + "/mkcert-v" + mkcertVersion + "-darwin-arm64",
	},
	"windows": {
		"amd64": "https://github.com/FiloSottile/mkcert/releases/download/v" + mkcertVersion + "/mkcert-v" + mkcertVersion + "-windows-amd64.exe",
	},
}

var cloudflaredURLs = map[string]map[string]string{
	"linux": {
		"amd64": "https://github.com/cloudflare/cloudflared/releases/download/" + cloudflaredVersion + "/cloudflared-linux-amd64",
		"arm64": "https://github.com/cloudflare/cloudflared/releases/download/" + cloudflaredVersion + "/cloudflared-linux-arm64",
	},
	"darwin": {
		"amd64": "https://github.com/cloudflare/cloudflared/releases/download/" + cloudflaredVersion + "/cloudflared-darwin-amd64.tgz",
	},
	"windows": {
		"amd64": "https://github.com/cloudflare/cloudflared/releases/download/" + cloudflaredVersion + "/cloudflared-windows-amd64.exe",
	},
}

// EnsureAll initiates concurrent fetching scaling and locally linking the required dependencies.
// It will instantly yield cache results, preventing unnecessary HTTP requests.
func EnsureAll(ctx context.Context) (ResolvedDeps, error) {
	var res ResolvedDeps
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	ensure := func(name string, urls map[string]map[string]string, dest *string) {
		defer wg.Done()

		osMap, ok := urls[runtime.GOOS]
		if !ok {
			mu.Lock()
			if firstErr == nil {
				firstErr = fmt.Errorf("the dependency %s is currently not supported for OS: %s", name, runtime.GOOS)
			}
			mu.Unlock()
			return
		}

		url, ok := osMap[runtime.GOARCH]
		if !ok {
			mu.Lock()
			if firstErr == nil {
				firstErr = fmt.Errorf("the dependency %s is currently not supported for Architecture: %s", name, runtime.GOARCH)
			}
			mu.Unlock()
			return
		}

		path, err := establishDependency(ctx, name, url)

		mu.Lock()
		if err != nil && firstErr == nil {
			firstErr = fmt.Errorf("failed resolving %s -> %w", name, err)
		} else if err == nil {
			*dest = path
		}
		mu.Unlock()
	}

	wg.Add(3)
	go ensure("caddy", caddyURLs, &res.Caddy)
	go ensure("mkcert", mkcertURLs, &res.Mkcert)
	go ensure("cloudflared", cloudflaredURLs, &res.Cloudflared)

	wg.Wait()
	return res, firstErr
}

func establishDependency(ctx context.Context, depName string, url string) (string, error) {
	// 1. System-First: Lookup globally installed dependencies immediately
	binNameWanted := depName
	if runtime.GOOS == "windows" && !strings.HasSuffix(binNameWanted, ".exe") {
		binNameWanted += ".exe"
	}

	if sysPath, err := exec.LookPath(binNameWanted); err == nil {
		return sysPath, nil
	}

	// 2. Local-Second: Orchestrate standard XDG structural verification
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("failed locating system cache directory: %v", err)
	}

	xrpBinDir := filepath.Join(cacheDir, "xrp", "bin")
	if err := os.MkdirAll(xrpBinDir, 0755); err != nil {
		return "", err
	}

	cachePath := filepath.Join(xrpBinDir, binNameWanted)
	if stat, err := os.Stat(cachePath); err == nil && !stat.IsDir() {
		// Valid binary natively accessible within local namespace
		return cachePath, nil
	}

	// 3. Fallback: Initialize HTTP stream transfer and extraction
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d rejected package acquisition for %s", resp.StatusCode, depName)
	}

	// Route payload to memory or temp storage for SHA256 stream analysis
	var checksum string
	switch depName {
	case "caddy":
		checksum = caddyChecksums[runtime.GOOS][runtime.GOARCH]
	case "mkcert":
		checksum = mkcertChecksums[runtime.GOOS][runtime.GOARCH]
	case "cloudflared":
		checksum = cloudflaredChecksums[runtime.GOOS][runtime.GOARCH]
	}

	var streamReader io.Reader = resp.Body

	if checksum != "" {
		// Intercept the stream entirely into RAM / Temp to perform a non-destructive verification
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		hash := sha256.Sum256(bodyBytes)
		computed := hex.EncodeToString(hash[:])
		if computed != checksum {
			return "", fmt.Errorf("CRITICAL: SHA256 checksum mismatch for %s. Expected %s, got %s", depName, checksum, computed)
		}
		
		// Map the payload back into a consumable reader for the extractors
		streamReader = bytes.NewReader(bodyBytes)
	}

	if strings.HasSuffix(url, ".zip") {
		return extractZip(streamReader, xrpBinDir, binNameWanted)
	} else if strings.HasSuffix(url, ".tar.gz") || strings.HasSuffix(url, ".tgz") {
		return extractTarGz(streamReader, xrpBinDir, binNameWanted)
	}
	return extractRaw(streamReader, cachePath)
}

func extractZip(src io.Reader, destDir, binName string) (string, error) {
	tmpFile, err := os.CreateTemp("", "xrp-download-*.zip")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()
	defer func() {
		tmpFile.Close()
		os.Remove(tmpPath)
	}()

	if _, err := io.Copy(tmpFile, src); err != nil {
		return "", err
	}
	tmpFile.Close()

	r, err := zip.OpenReader(tmpPath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Base(f.Name), binName) {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()

			outPath := filepath.Join(destDir, binName)
			outFile, err := os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
			if err != nil {
				return "", err
			}
			defer outFile.Close()

			if _, err := io.Copy(outFile, rc); err != nil {
				return "", err
			}
			return outPath, nil
		}
	}
	return "", fmt.Errorf("binary %s entirely missing from zip archive structure", binName)
}

func extractTarGz(src io.Reader, destDir, binName string) (string, error) {
	gzr, err := gzip.NewReader(src)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		if header.Typeflag == tar.TypeReg {
			if strings.EqualFold(filepath.Base(header.Name), binName) {
				outPath := filepath.Join(destDir, binName)
				outFile, err := os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
				if err != nil {
					return "", err
				}
				defer outFile.Close()

				if _, err := io.Copy(outFile, tr); err != nil {
					return "", err
				}
				return outPath, nil
			}
		}
	}
	return "", fmt.Errorf("binary %s ultimately missing from tar archive payload", binName)
}

func extractRaw(src io.Reader, outPath string) (string, error) {
	outFile, err := os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, src); err != nil {
		return "", err
	}
	return outPath, nil
}
