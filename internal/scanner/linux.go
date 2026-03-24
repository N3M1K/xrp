//go:build linux

package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type LinuxScanner struct{}

func (s *LinuxScanner) Scan() ([]Process, error) {
	var processes []Process

	// Parse /proc/net/tcp
	tcpData, err := os.ReadFile("/proc/net/tcp")
	if err != nil {
		return nil, err
	}

	// portHex -> inode
	portToInode := make(map[int]string)
	lines := strings.Split(string(tcpData), "\n")
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) >= 10 {
			localAddr := fields[1]
			parts := strings.Split(localAddr, ":")
			if len(parts) == 2 {
				portHex := parts[1]
				port, _ := strconv.ParseInt(portHex, 16, 32)
				inode := fields[9]
				portToInode[int(port)] = inode
			}
		}
	}

	// Walk through /proc/[pid]/fd -> match inode -> PID
	procDirs, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	inodeToPid := make(map[string]int)
	for _, dir := range procDirs {
		if !dir.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(dir.Name())
		if err != nil {
			continue
		}

		fdDir := fmt.Sprintf("/proc/%d/fd", pid)
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}

		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err == nil && strings.HasPrefix(link, "socket:[") && strings.HasSuffix(link, "]") {
				inode := link[8 : len(link)-1]
				inodeToPid[inode] = pid
			}
		}
	}

	for port, inode := range portToInode {
		if pid, ok := inodeToPid[inode]; ok {
			processName := ""
			cwd := ""

			// Read /proc/[pid]/comm
			comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
			if err == nil {
				processName = strings.TrimSpace(string(comm))
			}

			// Read /proc/[pid]/cwd symlink
			cwdStr, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid))
			if err == nil {
				cwd = cwdStr
			}

			knownApp := GetKnownApp(port)
			projectName := filepath.Base(cwd)

			processes = append(processes, Process{
				PID:         pid,
				Port:        port,
				ProcessName: processName,
				ProjectName: projectName,
				CWD:         cwd,
				KnownApp:    knownApp,
			})
		}
	}

	return processes, nil
}

func NewScanner() Scanner {
	return &LinuxScanner{}
}
