package scanner

import (
	"embed"
	"encoding/json"
	"os"
	"strconv"
)

//go:embed known-ports.json
var embeddedPorts embed.FS

var knownPortsCache map[int]KnownPort

// LoadKnownPorts loads port definitions from the given path or embedded file.
func LoadKnownPorts(path string) map[int]KnownPort {
	var data []byte
	var err error

	if path != "" {
		data, err = os.ReadFile(path)
	} else {
		data, err = embeddedPorts.ReadFile("known-ports.json")
	}

	if err != nil {
		return nil
	}

	var tempMap map[string]KnownPort
	if err := json.Unmarshal(data, &tempMap); err != nil {
		return nil
	}

	knownPortsCache = make(map[int]KnownPort)
	for k, v := range tempMap {
		if port, err := strconv.Atoi(k); err == nil {
			knownPortsCache[port] = v
		}
	}

	return knownPortsCache
}

// GetKnownApp returns the application name for a known port.
func GetKnownApp(port int) string {
	if knownPortsCache == nil {
		LoadKnownPorts("")
	}
	if portInfo, exists := knownPortsCache[port]; exists {
		return portInfo.Name
	}
	return ""
}
