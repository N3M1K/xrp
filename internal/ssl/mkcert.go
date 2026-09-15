package ssl

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// CertPair holds the paths to a certificate and its private key.
type CertPair struct {
	Cert string
	Key  string
}

func CheckMkcert() error {
	_, err := exec.LookPath("mkcert")
	if err != nil {
		return fmt.Errorf("mkcert not found in PATH. Please install mkcert to enable SSL")
	}
	return nil
}

// InstallTrustStore installs the mkcert CA into the system trust store.
//
// This must only be called from an interactive CLI context: on Linux/macOS the
// first install needs elevated privileges and mkcert will prompt for a sudo
// password. The daemon must never call this (it would hang waiting on a TTY).
func InstallTrustStore(mkcertPath string) error {
	if mkcertPath == "" {
		resolved, err := exec.LookPath("mkcert")
		if err != nil {
			return fmt.Errorf("mkcert not found in PATH: %w", err)
		}
		mkcertPath = resolved
	}

	cmd := exec.Command(mkcertPath, "-install")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mkcert -install failed: %w", err)
	}
	return nil
}

func getCertsDir() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	certsDir := filepath.Join(cacheDir, "halo", "certs")
	if err := os.MkdirAll(certsDir, 0755); err != nil {
		return "", err
	}
	return certsDir, nil
}

func fileHasContent(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Size() > 0
}

// normalizeHosts lowercases, trims and de-duplicates hostnames, sorted for a
// stable order.
func normalizeHosts(hosts []string) []string {
	seen := make(map[string]bool, len(hosts))
	out := make([]string, 0, len(hosts)+1)
	for _, h := range hosts {
		h = strings.ToLower(strings.TrimSpace(h))
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		out = append(out, h)
	}
	sort.Strings(out)
	return out
}

// HostsHash returns a stable fingerprint for a set of hostnames.
func HostsHash(hosts []string) string {
	sum := sha256.Sum256([]byte(strings.Join(normalizeHosts(hosts), ",")))
	return hex.EncodeToString(sum[:])
}

// EnsureCertForHosts makes sure a certificate covering every given hostname
// (plus localhost) exists on disk, regenerating it only when the set changes.
//
// A wildcard like *.localhost is NOT usable here: RFC-compliant verifiers
// (OpenSSL, Chrome, Firefox) reject wildcards whose suffix has no dot, which is
// exactly the case for single-label TLDs such as localhost, test, dev or media.
// So we list the actual hostnames as explicit SANs instead.
func EnsureCertForHosts(hosts []string, previousHash string) (CertPair, string, error) {
	certsDir, err := getCertsDir()
	if err != nil {
		return CertPair{}, "", err
	}

	certFile := filepath.Join(certsDir, "_hosts.pem")
	keyFile := filepath.Join(certsDir, "_hosts-key.pem")

	normalized := normalizeHosts(append([]string{"localhost"}, hosts...))
	hash := HostsHash(normalized)

	if hash == previousHash && fileHasContent(certFile) && fileHasContent(keyFile) {
		return CertPair{Cert: certFile, Key: keyFile}, hash, nil
	}

	args := append([]string{"-cert-file", certFile, "-key-file", keyFile}, normalized...)

	var out bytes.Buffer
	cmd := exec.Command("mkcert", args...)
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return CertPair{}, "", fmt.Errorf("failed to generate certificate for %v: %w\nOutput: %s", normalized, err, out.String())
	}
	if !fileHasContent(certFile) || !fileHasContent(keyFile) {
		return CertPair{}, "", fmt.Errorf("mkcert produced empty cert files for %v. Output: %s", normalized, out.String())
	}

	return CertPair{Cert: certFile, Key: keyFile}, hash, nil
}
