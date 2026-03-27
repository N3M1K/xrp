package socket

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// Send dials the socket, sends a Request and waits for Response
func Send(req Request) (*Response, error) {
	addr := GetSocketPath()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("xrp daemon is not running")
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(5 * time.Second))

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return &resp, nil
}
