//go:build linux

package proxy

import (
	"os"
	"os/exec"

	"golang.org/x/sys/unix"
)

// bindCapabilityPresent reports whether the file carries any
// security.capability xattr. halo only ever sets cap_net_bind_service on
// caddy, so a present xattr means the capability is already in place.
func bindCapabilityPresent(path string) bool {
	buf := make([]byte, 64)
	n, err := unix.Getxattr(path, "security.capability", buf)
	return err == nil && n > 0
}

// setBindCapability runs the one-time `sudo setcap` needed to bind 80/443.
// It inherits stdio so the user can answer the sudo password prompt.
func setBindCapability(path string) error {
	cmd := exec.Command("sudo", "setcap", "cap_net_bind_service=+ep", path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
