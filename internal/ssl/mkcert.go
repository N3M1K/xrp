package ssl

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/N3M1K/xrp/internal/config"
)

func CheckMkcert() error {
	_, err := exec.LookPath("mkcert")
	if err != nil {
		return fmt.Errorf("mkcert not found in PATH. Please install mkcert to enable SSL")
	}
	return nil
}

func InstallTrustStore() error {
	cmd := exec.Command("mkcert", "-install")
	// Capture output — mkcert may fail silently if no TTY
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("mkcert -install failed: %w\nOutput: %s", err, out.String())
	}
	return nil
}

func getCertsDir() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	certsDir := filepath.Join(cacheDir, "xrp", "certs")
	if err := os.MkdirAll(certsDir, 0755); err != nil {
		return "", err
	}
	return certsDir, nil
}

func GetCertPaths(tld string) (certPath string, keyPath string, err error) {
	certsDir, err := getCertsDir()
	if err != nil {
		return "", "", err
	}

	cleanTld := strings.TrimPrefix(tld, ".")
	certFile := filepath.Join(certsDir, fmt.Sprintf("_wildcard.%s.pem", cleanTld))
	keyFile := filepath.Join(certsDir, fmt.Sprintf("_wildcard.%s-key.pem", cleanTld))

	return certFile, keyFile, nil
}

func GenerateCert(tld string) (certPath string, keyPath string, err error) {
	certsDir, err := getCertsDir()
	if err != nil {
		return "", "", err
	}

	cleanTld := strings.TrimPrefix(tld, ".")
	wildcard := fmt.Sprintf("*.%s", cleanTld)
	certFile := filepath.Join(certsDir, fmt.Sprintf("_wildcard.%s.pem", cleanTld))
	keyFile := filepath.Join(certsDir, fmt.Sprintf("_wildcard.%s-key.pem", cleanTld))

	// Check if certificates already exist AND are non-empty (daemon can write 0-byte files on TTY failure)
	certOK := fileHasContent(certFile)
	keyOK := fileHasContent(keyFile)
	if certOK && keyOK {
		return certFile, keyFile, nil
	}

	// Remove stale/corrupt files before regenerating
	os.Remove(certFile)
	os.Remove(keyFile)

	var out bytes.Buffer
	cmd := exec.Command("mkcert", "-cert-file", certFile, "-key-file", keyFile, wildcard)
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("failed to generate cert for %s: %w\nOutput: %s", wildcard, err, out.String())
	}

	// Validate output files are non-empty
	if !fileHasContent(certFile) || !fileHasContent(keyFile) {
		return "", "", fmt.Errorf("mkcert ran but produced empty cert files for %s. Output: %s", wildcard, out.String())
	}

	return certFile, keyFile, nil
}

func fileHasContent(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Size() > 0
}

// CertPair holds the paths to a cert and key file for a specific TLD.
type CertPair struct {
	Cert string
	Key  string
}

// GenerateAllCerts generates wildcard certificates for every unique TLD
// present in the config (default TLD + all project-specific overrides).
func GenerateAllCerts(cfg *config.Config) ([]CertPair, error) {
	// Collect all unique TLDs
	seen := make(map[string]bool)
	tlds := []string{}

	addTLD := func(tld string) {
		key := strings.TrimPrefix(tld, ".")
		if key != "" && !seen[key] {
			seen[key] = true
			tlds = append(tlds, key)
		}
	}

	addTLD(cfg.TLD)
	for _, tld := range cfg.ProjectTLDs {
		addTLD(tld)
	}

	var pairs []CertPair
	for _, tld := range tlds {
		cert, key, err := GenerateCert(tld)
		if err != nil {
			// Non-fatal: log and continue so other TLDs still work
			fmt.Fprintf(os.Stderr, "[ssl] warning: failed to generate cert for *.%s: %v\n", tld, err)
			continue
		}
		pairs = append(pairs, CertPair{Cert: cert, Key: key})
	}
	return pairs, nil
}
