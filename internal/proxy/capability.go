package proxy

import (
	"github.com/N3M1K/halo-proxy/internal/config"
)

// NeedsBindCapability reports whether caddy still needs the
// cap_net_bind_service capability to bind the configured privileged ports, and
// returns the resolved caddy binary path.
func NeedsBindCapability(cfg *config.Config) (bool, string) {
	if cfg.HTTPPort >= 1024 && cfg.HTTPSPort >= 1024 {
		return false, ""
	}
	path, err := resolveCaddyBinary()
	if err != nil {
		return false, ""
	}
	return !bindCapabilityPresent(path), path
}

// GrantBindCapability applies the platform-specific fix that lets caddy bind
// privileged ports. On Linux this runs `sudo setcap cap_net_bind_service=+ep`.
func GrantBindCapability(path string) error {
	return setBindCapability(path)
}
