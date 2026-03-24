//go:build windows

package scanner

type WindowsScanner struct{}

func (s *WindowsScanner) Scan() ([]Process, error) {
	// Dummy implementation for Windows
	// In MVP we only fully support Linux and Darwin
	return nil, nil
}

func NewScanner() Scanner {
	return &WindowsScanner{}
}
