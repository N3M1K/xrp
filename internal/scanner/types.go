package scanner

// Process represents a running network process.
type Process struct {
	PID         int
	Port        int
	ProcessName string
	ProjectName string
	CWD         string
	KnownApp    string
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
