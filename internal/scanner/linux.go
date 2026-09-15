//go:build linux

package scanner

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type LinuxScanner struct{}

// listenSocket is one LISTEN socket entry parsed from /proc/net/tcp[6].
type listenSocket struct {
	port  int
	inode string
	addr  string
}

func (s *LinuxScanner) Scan() ([]Process, error) {
	sockets := []listenSocket{}

	for _, file := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		data, err := os.ReadFile(file)
		if err != nil {
			// tcp6 may be unavailable (e.g. IPv6 disabled) — not fatal.
			continue
		}
		sockets = append(sockets, parseProcNet(data)...)
	}

	if len(sockets) == 0 {
		return nil, nil
	}

	inodeToPid := buildInodeToPid()

	var processes []Process
	seen := make(map[string]bool)

	for _, sock := range sockets {
		pid, ok := inodeToPid[sock.inode]
		if !ok {
			continue
		}

		// De-duplicate: a process listening on the same port over both IPv4 and
		// IPv6 would otherwise show up twice.
		key := fmt.Sprintf("%d:%d", pid, sock.port)
		if seen[key] {
			continue
		}
		seen[key] = true

		processName := ""
		cwd := ""

		if comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid)); err == nil {
			processName = strings.TrimSpace(string(comm))
		}
		if cwdStr, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid)); err == nil {
			cwd = cwdStr
		}

		processes = append(processes, Process{
			PID:         pid,
			Port:        sock.port,
			ProcessName: processName,
			ProjectName: filepath.Base(cwd),
			CWD:         cwd,
			KnownApp:    GetKnownApp(sock.port),
			Addr:        sock.addr,
		})
	}

	return processes, nil
}

// parseProcNet parses /proc/net/tcp or /proc/net/tcp6 content.
func parseProcNet(data []byte) []listenSocket {
	var out []listenSocket

	lines := strings.Split(string(data), "\n")
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		if fields[3] != "0A" { // 0A = TCP_LISTEN
			continue
		}

		localAddr := fields[1]
		parts := strings.Split(localAddr, ":")
		if len(parts) != 2 {
			continue
		}

		port64, err := strconv.ParseInt(parts[1], 16, 32)
		if err != nil {
			continue
		}

		out = append(out, listenSocket{
			port:  int(port64),
			inode: fields[9],
			addr:  decodeProcAddr(parts[0]),
		})
	}
	return out
}

// buildInodeToPid walks /proc and maps socket inodes to owning PIDs.
func buildInodeToPid() map[string]int {
	inodeToPid := make(map[string]int)

	procDirs, err := os.ReadDir("/proc")
	if err != nil {
		return inodeToPid
	}

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
				inodeToPid[link[8:len(link)-1]] = pid
			}
		}
	}

	return inodeToPid
}

// decodeProcAddr converts the hex address field from /proc/net/tcp[6] into a
// dialable loopback host. Wildcard binds are normalised to 127.0.0.1.
func decodeProcAddr(hexAddr string) string {
	switch len(hexAddr) {
	case 8: // IPv4, 4 bytes little-endian
		b := make([]byte, 4)
		for i := 0; i < 4; i++ {
			v, err := strconv.ParseUint(hexAddr[i*2:i*2+2], 16, 8)
			if err != nil {
				return "127.0.0.1"
			}
			b[3-i] = byte(v)
		}
		ip := net.IP(b)
		if ip.IsUnspecified() {
			return "127.0.0.1"
		}
		return ip.String()

	case 32: // IPv6, 4 little-endian 32-bit words
		b := make([]byte, 16)
		for w := 0; w < 4; w++ {
			word := hexAddr[w*8 : (w+1)*8]
			for i := 0; i < 4; i++ {
				v, err := strconv.ParseUint(word[i*2:i*2+2], 16, 8)
				if err != nil {
					return "127.0.0.1"
				}
				b[w*4+(3-i)] = byte(v)
			}
		}
		ip := net.IP(b)
		if ip.IsUnspecified() {
			// Dual-stack wildcard; 127.0.0.1 is reachable in the common case.
			return "127.0.0.1"
		}
		if ip.IsLoopback() {
			return "::1"
		}
		return ip.String()
	}

	return "127.0.0.1"
}

func NewScanner() Scanner {
	return &LinuxScanner{}
}
