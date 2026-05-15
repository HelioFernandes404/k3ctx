package tunnel

import (
	"crypto/md5" //nolint:gosec
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// GetUniquePort generates a deterministic port within [start, start+size).
// Uses the first 4 hex chars of MD5(contextName) for compatibility with the Python implementation.
func GetUniquePort(contextName string, start, size int) int {
	h := md5.Sum([]byte(contextName)) //nolint:gosec
	// Use first 2 bytes (4 hex chars) as uint16
	n := binary.BigEndian.Uint16(h[:2])
	return start + (int(n) % size)
}

// GetTunnelPIDFile returns the PID file path and ensures the state dir exists.
func GetTunnelPIDFile(contextName, stateDir string) string {
	_ = os.MkdirAll(stateDir, 0o755)
	return filepath.Join(stateDir, contextName+".pid")
}

// IsTunnelRunning checks whether the SSH tunnel process for contextName is alive.
func IsTunnelRunning(contextName, stateDir string) bool {
	pidFile := GetTunnelPIDFile(contextName, stateDir)
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		_ = os.Remove(pidFile)
		return false
	}
	// Signal 0 checks existence without killing
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		_ = os.Remove(pidFile)
		return false
	}
	return true
}

// KillTunnel sends SIGTERM to the tunnel process and removes its PID file.
func KillTunnel(contextName, stateDir string) {
	pidFile := GetTunnelPIDFile(contextName, stateDir)
	defer func() { _ = os.Remove(pidFile) }()

	data, err := os.ReadFile(pidFile)
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = proc.Signal(syscall.SIGTERM)
}

// KillAllTunnels kills every tunnel found in stateDir.
func KillAllTunnels(stateDir string) {
	entries, err := filepath.Glob(filepath.Join(stateDir, "*.pid"))
	if err != nil || entries == nil {
		return
	}
	for _, pidFile := range entries {
		base := filepath.Base(pidFile)
		ctx := strings.TrimSuffix(base, ".pid")
		KillTunnel(ctx, stateDir)
	}
}

// CreateTunnelOptions holds optional SSH parameters for CreateTunnel.
type CreateTunnelOptions struct {
	Username    string
	KeyFilename string
	Port        int
	ProxyCmd    string
}

// CreateTunnel opens a background SSH port-forward and returns the tunnel PID.
// Returns nil PID when pgrep cannot locate the process (not an error).
func CreateTunnel(sshHost, internalIP string, localPort, remotePort int, opts CreateTunnelOptions) (*int, error) {
	cmd := []string{
		"ssh", "-f", "-N",
		"-o", "ExitOnForwardFailure=yes",
		"-o", "ServerAliveInterval=60",
	}
	if opts.ProxyCmd != "" {
		cmd = append(cmd, "-o", "ProxyCommand="+opts.ProxyCmd)
	}
	if opts.KeyFilename != "" {
		cmd = append(cmd, "-i", opts.KeyFilename)
	}
	if opts.Port != 0 && opts.Port != 22 {
		cmd = append(cmd, "-p", strconv.Itoa(opts.Port))
	}
	if opts.Username != "" {
		cmd = append(cmd, "-l", opts.Username)
	}
	cmd = append(cmd,
		"-L", fmt.Sprintf("%d:%s:%d", localPort, internalIP, remotePort),
		sshHost,
	)

	out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH tunnel: %s", string(out))
	}

	time.Sleep(500 * time.Millisecond)

	pattern := fmt.Sprintf("ssh.*%d:%s:%d", localPort, internalIP, remotePort)
	pgrepOut, err := exec.Command("pgrep", "-f", pattern).Output()
	if err != nil || len(strings.TrimSpace(string(pgrepOut))) == 0 {
		return nil, nil
	}
	pid, err := strconv.Atoi(strings.Fields(string(pgrepOut))[0])
	if err != nil {
		return nil, nil
	}
	return &pid, nil
}

// SaveTunnelPID writes the PID to the tunnel PID file. No-op when pid is nil.
func SaveTunnelPID(contextName string, pid *int, stateDir string) {
	if pid == nil {
		return
	}
	pidFile := GetTunnelPIDFile(contextName, stateDir)
	_ = os.WriteFile(pidFile, []byte(strconv.Itoa(*pid)), 0o644)
}
