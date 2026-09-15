//go:build !linux

package proxy

// On Windows/macOS there is no per-binary capability model: privileged ports
// require running elevated (Administrator / root). Report the capability as
// present so `halo start` does not try to fix it automatically.
func bindCapabilityPresent(string) bool { return true }

func setBindCapability(string) error { return nil }
