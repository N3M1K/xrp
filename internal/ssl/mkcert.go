package ssl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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

	// Check if certificates already exist
	if _, err := os.Stat(certFile); err == nil {
		if _, err := os.Stat(keyFile); err == nil {
			return certFile, keyFile, nil
		}
	}

	cmd := exec.Command("mkcert", "-cert-file", certFile, "-key-file", keyFile, wildcard)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("failed to generate certs: %w", err)
	}

	return certFile, keyFile, nil
}
