package scanner

// Process represents a running network process.
type Process struct {
	PID         int
	Port        int
	ProcessName string
	ProjectName string
	CWD         string
	KnownApp    string
	// Addr is the concrete loopback address the service is reachable on
	// (e.g. "127.0.0.1" or "::1"). It is used as the reverse-proxy upstream host.
	Addr string
	// URL is the friendly HTTPS URL for this service, filled in by the daemon
	// once the effective TLD is known.
	URL string
	// TunnelURL is set when a cloudflared tunnel is active for this project.
	TunnelURL string
}

// KnownPort represents metadata for a recognized port.
type KnownPort struct {
	Name     string `json:"name"`
	Category string `json:"category"`
}

// Scanner defines the interface for scanning running network processes.
type Scanner interface {
	Scan() ([]Process, error)
}
