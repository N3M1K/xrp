//go:build windows

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func isProcessRunning(pid int) bool {
	_, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	out, _ := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH").Output()
	return strings.Contains(string(out), fmt.Sprintf("%d", pid))
}
