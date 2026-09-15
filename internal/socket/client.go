package socket

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// Send dials the socket, sends a Request and waits for Response using the
// default timeout.
func Send(req Request) (*Response, error) {
	return SendWithTimeout(req, 10*time.Second)
}

// SendWithTimeout is like Send but allows callers to pick a deadline. Commands
// such as `share` (which waits for cloudflared to publish a URL) need a much
// longer window than simple cache lookups.
func SendWithTimeout(req Request, timeout time.Duration) (*Response, error) {
	addr := GetSocketPath()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, fmt.Errorf("xrp daemon is not running")
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(timeout))

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return &resp, nil
}
