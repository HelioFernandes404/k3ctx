package infrastructure

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/systemframe/k3ctx/internal/domain"
	"github.com/systemframe/k3ctx/internal/kubeconfig"
	"github.com/systemframe/k3ctx/internal/paths"
	sshpkg "github.com/systemframe/k3ctx/internal/ssh"
	"github.com/systemframe/k3ctx/internal/tunnel"
)

// defaultCheckAlreadyConnected returns cached ConnectionArtifacts when the tunnel is already live and the
// kubeconfig context exists, allowing Connect to skip SSH, remote kubeconfig fetch, and API verification.
func defaultCheckAlreadyConnected(contextName, stateDir string, cfg domain.EffectiveConfig) (domain.ConnectionArtifacts, bool) {
	localPort := tunnel.GetUniquePort(contextName, cfg.PortRangeStart, cfg.PortRangeSize)
	if tunnel.Liveness(contextName, stateDir, localPort) != "live" {
		return domain.ConnectionArtifacts{}, false
	}
	if !kubeconfig.ContextExists(contextName) {
		return domain.ConnectionArtifacts{}, false
	}
	params, err := tunnel.LoadConnParams(contextName, stateDir)
	if err != nil {
		return domain.ConnectionArtifacts{}, false
	}
	pid := readTunnelPID(contextName, stateDir)
	return domain.ConnectionArtifacts{
		LocalPort:                localPort,
		InternalIP:               params.InternalIP,
		TunnelPID:                pid,
		UsedCache:                true,
		AlreadyConnected:         true,
		ArgocdLocalPort:          secondaryLivePort(contextName+"-argocd", stateDir, argocdPortRangeStart, argocdPortRangeSize),
		AlertmanagerLocalPort:    secondaryLivePort(contextName+"-alertmanager", stateDir, alertmanagerPortRangeStart, alertmanagerPortRangeSize),
		VictoriaMetricsLocalPort: secondaryLivePort(contextName+"-victoriametrics", stateDir, victoriaMetricsPortRangeStart, victoriaMetricsPortRangeSize),
	}, true
}

// secondaryLivePort returns a pointer to the deterministic local port for a secondary tunnel context
// when that tunnel process is running and its port accepts connections, or nil otherwise.
func secondaryLivePort(contextName, stateDir string, portStart, portSize int) *int {
	if !tunnel.IsTunnelRunning(contextName, stateDir) {
		return nil
	}
	port := tunnel.GetUniquePort(contextName, portStart, portSize)
	if !tunnel.IsPortLive(port, 2*time.Second) {
		return nil
	}
	return &port
}

// defaultSSHConnect resolves the SSH connection target from inventory + ssh config and returns a remote runner.
func defaultSSHConnect(target domain.ClusterTarget, cfg domain.EffectiveConfig) (string, string, *string, int, *string, func(string) (string, error), error) {
	sshCfg := sshpkg.LoadSSHConfig(target.HostAlias(), cfg.SSHConfigPath)
	hostname, username, keyfile, sshPort, proxycmd, err := sshpkg.ResolveConnectionTarget(
		target.HostAlias(), sshCfg, cfg.SSHKeyPath, target.HostConfig(), cfg.SSHUser,
	)
	if err != nil {
		return "", "", nil, 0, nil, nil, err
	}
	runner := sshpkg.MakeRemoteRunner(hostname, username, keyfile, sshPort, proxycmd)
	return hostname, username, keyfile, sshPort, proxycmd, runner, nil
}

// defaultPrepareKubeconfig fetches the remote k3s kubeconfig, rewrites the server, and picks a local port.
func defaultPrepareKubeconfig(runner func(string) (string, error), target domain.ClusterTarget, cfg domain.EffectiveConfig) (string, int, bool, string, error) {
	internalIP, err := sshpkg.GetInternalIP(runner)
	if err != nil {
		return "", 0, false, "", fmt.Errorf("get internal IP: %w", err)
	}
	cachePath := paths.KubeconfigCachePath(target.ContextName())
	content, usedCache, err := sshpkg.FetchRemoteFileCached(runner, cfg.RemoteK3sConfigPath, cachePath)
	if err != nil {
		return "", 0, false, "", fmt.Errorf("fetch kubeconfig: %w", err)
	}
	localPort := tunnel.GetUniquePort(target.ContextName(), cfg.PortRangeStart, cfg.PortRangeSize)
	updated, err := kubeconfig.UpdateKubeconfigServer(content, internalIP, cfg.K3sAPIPort, true, localPort)
	if err != nil {
		return "", 0, false, "", fmt.Errorf("update kubeconfig server: %w", err)
	}
	return internalIP, localPort, usedCache, updated, nil
}

// defaultEnsureTunnel opens an SSH tunnel for contextName if one isn't already running, and persists conn params.
func defaultEnsureTunnel(contextName, hostname, internalIP, username string, keyfile *string, sshPort, localPort, k3sPort int, proxycmd *string, stateDir string) (*int, bool, error) {
	if tunnel.IsTunnelRunning(contextName, stateDir) {
		pid := readTunnelPID(contextName, stateDir)
		return pid, true, nil
	}
	opts := tunnel.CreateTunnelOptions{Username: username, Port: sshPort}
	if keyfile != nil {
		opts.KeyFilename = *keyfile
	}
	if proxycmd != nil {
		opts.ProxyCmd = *proxycmd
	}
	pid, err := tunnel.CreateTunnel(hostname, internalIP, localPort, k3sPort, opts)
	if err != nil {
		return nil, false, fmt.Errorf("create tunnel: %w", err)
	}
	tunnel.SaveTunnelPID(contextName, pid, stateDir)
	keyFileStr := ""
	if keyfile != nil {
		keyFileStr = *keyfile
	}
	proxyCmdStr := ""
	if proxycmd != nil {
		proxyCmdStr = *proxycmd
	}
	_ = tunnel.SaveConnParams(contextName, stateDir, tunnel.ConnParams{
		SSHHost:    hostname,
		InternalIP: internalIP,
		LocalPort:  localPort,
		RemotePort: k3sPort,
		Username:   username,
		KeyFile:    keyFileStr,
		SSHPort:    sshPort,
		ProxyCmd:   proxyCmdStr,
	})
	return pid, false, nil
}

func readTunnelPID(contextName, stateDir string) *int {
	pidFile := tunnel.GetTunnelPIDFile(contextName, stateDir)
	data, err := os.ReadFile(pidFile) //nolint:gosec // pidFile path is built from validated stateDir
	if err != nil {
		return nil
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return nil
	}
	return &pid
}
