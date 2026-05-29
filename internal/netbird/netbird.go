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
	Status string       `json:"status"`
	Peers  peersSection `json:"peers"`
}

// StatusResult holds the parsed output of `netbird status --json`.
type StatusResult struct {
	DaemonStatus string
	Peers        []Peer
}

// ParseStatus decodes the JSON output of `netbird status --json` into a StatusResult.
func ParseStatus(data []byte) (StatusResult, error) {
	var out statusOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return StatusResult{}, fmt.Errorf("parse netbird status: %w", err)
	}
	return StatusResult{DaemonStatus: out.Status, Peers: out.Peers.Details}, nil
}

// ParsePeers decodes the JSON output of `netbird status --json`.
func ParsePeers(data []byte) ([]Peer, error) {
	r, err := ParseStatus(data)
	if err != nil {
		return nil, err
	}
	return r.Peers, nil
}

// RunStatus executes `netbird status --json` and returns raw JSON output.
func RunStatus(binPath string) ([]byte, error) {
	out, err := exec.Command(binPath, "status", "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("netbird CLI unavailable or not authenticated: %w", err)
	}
	return out, nil
}
