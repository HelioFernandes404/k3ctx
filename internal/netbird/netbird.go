package netbird

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

// Peer represents a single NetBird peer from `netbird status --json`.
type Peer struct {
	FQDN   string `json:"fqdn"`
	IP     string `json:"netbirdIp"`
	Status string `json:"status"` // "Connected", "Connecting", "Idle"
}

type peersSection struct {
	Details []Peer `json:"details"`
}

type statusOutput struct {
	Peers peersSection `json:"peers"`
}

// ParsePeers decodes the JSON output of `netbird status --json`.
func ParsePeers(data []byte) ([]Peer, error) {
	var out statusOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("parse netbird status: %w", err)
	}
	return out.Peers.Details, nil
}

// RunStatus executes `netbird status --json` and returns raw JSON output.
func RunStatus(binPath string) ([]byte, error) {
	out, err := exec.Command(binPath, "status", "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("netbird CLI unavailable or not authenticated: %w", err)
	}
	return out, nil
}
