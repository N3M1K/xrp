package socket

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/N3M1K/xrp/internal/scanner"
	"github.com/N3M1K/xrp/internal/tunnel"
)

var (
	cachedProcesses []scanner.Process
	cacheMutex      sync.RWMutex
	shutdownFunc    func()
)

// SetShutdownHandler registers a callback invoked when a client sends the
// "shutdown" command. This gives `xrp stop` a graceful, cross-platform way to
// ask the daemon to exit (Windows has no usable SIGINT for background procs).
func SetShutdownHandler(f func()) {
	shutdownFunc = f
}

// UpdateProcesses safely stores the latest scanned processes array in the socket server state
func UpdateProcesses(processes []scanner.Process) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	// Create a safe copy
	cachedProcesses = make([]scanner.Process, len(processes))
	copy(cachedProcesses, processes)
}

// TCP: required for Tauri (Rust) GUI compatibility on Windows
func GetSocketPath() string {
	return "127.0.0.1:40192"
}

func StartServer(logger *log.Logger) error {
	addr := GetSocketPath()

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("could not listen on tcp socket: %w", err)
	}

	logger.Printf("Socket tcp server listening on %s", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Printf("Accept err: %v", err)
			continue
		}

		go handleConnection(conn, logger)
	}
}

func handleConnection(conn net.Conn, logger *log.Logger) {
	defer conn.Close()

	// Bound only the request read; long-running commands (e.g. share) may take
	// much longer than the initial handshake.
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	var req Request
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&req); err != nil {
		sendError(conn, "Invalid JSON payload")
		return
	}
	conn.SetDeadline(time.Time{})

	switch req.Cmd {
	case "list":
		cacheMutex.RLock()
		data := cachedProcesses
		cacheMutex.RUnlock()
		if data == nil {
			data = []scanner.Process{}
		}
		sendSuccess(conn, data)

	case "status":
		sendSuccess(conn, "running")

	case "shutdown":
		sendSuccess(conn, "stopping")
		if shutdownFunc != nil {
			go shutdownFunc()
		}

	case "open":
		url := req.Args["url"]
		if url == "" {
			sendError(conn, "Missing url parameter")
			return
		}
		if err := openBrowser(url); err != nil {
			sendError(conn, fmt.Sprintf("Failed to open browser: %v", err))
		} else {
			sendSuccess(conn, "opened")
		}

	case "share":
		project := req.Args["project"]
		portStr := req.Args["port"]
		port, err := strconv.Atoi(portStr)
		if err != nil || project == "" {
			sendError(conn, "Invalid project or port")
			return
		}

		// Prefer the exact bind address the service was discovered on.
		host := "127.0.0.1"
		cacheMutex.RLock()
		for _, p := range cachedProcesses {
			if p.ProjectName == project && p.Port == port && p.Addr != "" {
				host = p.Addr
				break
			}
		}
		cacheMutex.RUnlock()

		url, err := tunnel.StartTunnel(host, port, project)
		if err != nil {
			sendError(conn, err.Error())
			return
		}
		sendSuccess(conn, url)

	case "unshare":
		project := req.Args["project"]
		if err := tunnel.StopTunnel(project); err != nil {
			sendError(conn, err.Error())
			return
		}
		sendSuccess(conn, "stopped")

	default:
		sendError(conn, "Unknown command")
	}
}

func sendSuccess(conn net.Conn, data any) {
	b, _ := json.Marshal(data)
	resp := Response{Success: true, Data: b}
	json.NewEncoder(conn).Encode(resp)
}

func sendError(conn net.Conn, errMsg string) {
	resp := Response{Success: false, Error: errMsg}
	json.NewEncoder(conn).Encode(resp)
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return fmt.Errorf("unsupported platform")
	}
}
