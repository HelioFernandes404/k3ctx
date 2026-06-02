package infrastructure

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/systemframe/k3ctx/internal/domain"
	"github.com/systemframe/k3ctx/internal/kubeconfig"
	"github.com/systemframe/k3ctx/internal/paths"
	sshpkg "github.com/systemframe/k3ctx/internal/ssh"
	"github.com/systemframe/k3ctx/internal/tunnel"
)

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
