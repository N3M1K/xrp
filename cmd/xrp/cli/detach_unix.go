//go:build !windows

package cli

import "syscall"

// detachAttr starts the daemon in its own session so it survives the parent
// terminal/shell exiting (and never receives SIGHUP when the terminal closes).
func detachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
