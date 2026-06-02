package tunnel

import (
	"context"
	"crypto/md5" //nolint:gosec
	"encoding/binary"
	"fmt"
	"net"
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
	_ = os.MkdirAll(stateDir, 0o700)
	return filepath.Join(stateDir, contextName+".pid")
}

// IsTunnelRunning checks whether the SSH tunnel process for contextName is alive.
func IsTunnelRunning(contextName, stateDir string) bool {
	pidFile := GetTunnelPIDFile(contextName, stateDir)
	data, err := os.ReadFile(pidFile) //nolint:gosec // pidFile built from validated stateDir
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

	data, err := os.ReadFile(pidFile) //nolint:gosec // pidFile built from validated stateDir
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

// buildSSHTunnelArgs builds the argv for `ssh -f -N` background port-forward.
func buildSSHTunnelArgs(sshHost, internalIP string, localPort, remotePort int, opts CreateTunnelOptions) []string {
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
	return cmd
}

// CreateTunnel opens a background SSH port-forward and returns the tunnel PID.
// Returns nil PID when pgrep cannot locate the process (not an error).
func CreateTunnel(sshHost, internalIP string, localPort, remotePort int, opts CreateTunnelOptions) (*int, error) {
	cmd := buildSSHTunnelArgs(sshHost, internalIP, localPort, remotePort, opts)

	if _, err := exec.CommandContext(context.Background(), cmd[0], cmd[1:]...).CombinedOutput(); err != nil { //nolint:gosec // SSH command built from validated config
		return nil, fmt.Errorf("SSH tunnel process failed: %w", err)
	}

	time.Sleep(500 * time.Millisecond)

	pattern := fmt.Sprintf("ssh.*%d:%s:%d", localPort, internalIP, remotePort)
	pgrepOut, err := exec.CommandContext(context.Background(), "pgrep", "-f", pattern).Output() //nolint:gosec // pgrep pattern built from numeric ports and validated IP
	if err != nil || len(strings.TrimSpace(string(pgrepOut))) == 0 {
		return nil, nil
	}
	pid, err := strconv.Atoi(strings.Fields(string(pgrepOut))[0])
	if err != nil {
		return nil, nil
	}
	return &pid, nil
}

// CreateKubectlPortForward opens a background kubectl port-forward and returns the process PID.
// Returns nil PID when pgrep cannot locate the process (not an error).
func CreateKubectlPortForward(contextName, namespace, serviceName string, localPort, remotePort int) (*int, error) {
	cmd := exec.CommandContext( //nolint:gosec // kubectl with validated args
		context.Background(),
		"kubectl", "port-forward",
		"-n", namespace,
		"--context", contextName,
		"svc/"+serviceName,
		fmt.Sprintf("%d:%d", localPort, remotePort),
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("kubectl port-forward process failed: %w", err)
	}
	_ = cmd.Process.Release()

	time.Sleep(500 * time.Millisecond)

	pattern := fmt.Sprintf("kubectl.*port-forward.*%d:%d", localPort, remotePort)
	pgrepOut, err := exec.CommandContext(context.Background(), "pgrep", "-f", pattern).Output() //nolint:gosec // pgrep pattern built from numeric ports
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
	_ = os.WriteFile(pidFile, []byte(strconv.Itoa(*pid)), 0o600)
}

// IsPortLive reports whether localhost:<localPort> accepts a TCP connection within timeout.
func IsPortLive(localPort int, timeout time.Duration) bool {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(context.Background(), "tcp", fmt.Sprintf("localhost:%d", localPort))
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// Liveness returns the liveness state of a managed tunnel:
//   - "live"  — process running and local port responds
//   - "stale" — process running but local port is unresponsive
//   - "dead"  — no running process (PID file absent or process gone)
func Liveness(contextName, stateDir string, localPort int) string {
	if !IsTunnelRunning(contextName, stateDir) {
		return "dead"
	}
	if IsPortLive(localPort, 2*time.Second) {
		return "live"
	}
	return "stale"
}
