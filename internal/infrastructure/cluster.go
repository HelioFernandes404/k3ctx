package infrastructure

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/systemframe/k3ctx/internal/domain"
	"github.com/systemframe/k3ctx/internal/kubeconfig"
	"github.com/systemframe/k3ctx/internal/tunnel"
)

// LocalClusterConnector implements domain.ClusterConnector using SSH tunnels.
type LocalClusterConnector struct {
	VerifyAPIReady         bool
	APIReadyTimeout        time.Duration
	APIReadyInterval       time.Duration
	StateDir               string
	ArgocdAdapter          domain.ArgocdConnector
	AlertmanagerAdapter    domain.AlertmanagerConnector
	VictoriaMetricsAdapter domain.VictoriaMetricsConnector

	// Injectable for testing:
	checkAlreadyConnected func(contextName, stateDir string, cfg domain.EffectiveConfig) (domain.ConnectionArtifacts, bool)
	sshConnect            func(target domain.ClusterTarget, cfg domain.EffectiveConfig) (hostname, username string, keyfile *string, sshPort int, proxycmd *string, runner func(string) (string, error), err error)
	prepareKubeconfig     func(runner func(string) (string, error), target domain.ClusterTarget, cfg domain.EffectiveConfig) (internalIP string, localPort int, usedCache bool, content string, err error)
	ensureTunnel          func(contextName, hostname, internalIP, username string, keyfile *string, sshPort, localPort, k3sPort int, proxycmd *string, stateDir string) (pid *int, reused bool, err error)
	killTunnel            func(contextName, stateDir string)
	pollAPIReady          func(localPort int, kubeconfigText string, timeout, interval time.Duration) error
	mergeKubeconfig       func(content, contextName string) (string, error)
}

// NewLocalClusterConnector returns a connector with real subprocess defaults.
func NewLocalClusterConnector(argocd domain.ArgocdConnector, alertmanager domain.AlertmanagerConnector, victoriaMetrics domain.VictoriaMetricsConnector) *LocalClusterConnector {
	verifyAPI := true
	if v := os.Getenv("K3CTX_VERIFY_API_READY"); v != "" {
		v = strings.ToLower(strings.TrimSpace(v))
		verifyAPI = v != "0" && v != "false" && v != "no" && v != "off"
	}
	timeout := 15 * time.Second
	if v := os.Getenv("K3CTX_API_READY_TIMEOUT_SECONDS"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			timeout = time.Duration(f * float64(time.Second))
		}
	}

	c := &LocalClusterConnector{
		VerifyAPIReady:         verifyAPI,
		APIReadyTimeout:        timeout,
		APIReadyInterval:       250 * time.Millisecond,
		ArgocdAdapter:          argocd,
		AlertmanagerAdapter:    alertmanager,
		VictoriaMetricsAdapter: victoriaMetrics,
	}
	c.checkAlreadyConnected = defaultCheckAlreadyConnected
	c.sshConnect = defaultSSHConnect
	c.prepareKubeconfig = defaultPrepareKubeconfig
	c.ensureTunnel = defaultEnsureTunnel
	c.killTunnel = tunnel.KillTunnel
	c.pollAPIReady = defaultPollAPIReady
	c.mergeKubeconfig = kubeconfig.MergeKubeconfig
	return c
}

func (c *LocalClusterConnector) stateDir() string {
	if c.StateDir != "" {
		return c.StateDir
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "state", "k3ctx-tunnels")
}

// Connect implements domain.ClusterConnector.
func (c *LocalClusterConnector) Connect(
	target domain.ClusterTarget,
	cfg domain.EffectiveConfig,
	_ domain.NetworkRequirement,
) (domain.ConnectionArtifacts, error) {
	stateDir := c.stateDir()

	if c.checkAlreadyConnected != nil {
		if artifacts, ok := c.checkAlreadyConnected(target.ContextName(), stateDir, cfg); ok {
			return artifacts, nil
		}
	}

	hostname, username, keyfile, sshPort, proxycmd, runner, err := c.sshConnect(target, cfg)
	if err != nil {
		return domain.ConnectionArtifacts{}, err
	}

	internalIP, localPort, usedCache, content, err := c.prepareKubeconfig(runner, target, cfg)
	if err != nil {
		return domain.ConnectionArtifacts{}, err
	}

	tunnelPID, tunnelReused, err := c.ensureTunnel(
		target.ContextName(), hostname, internalIP, username,
		keyfile, sshPort, localPort, cfg.K3sAPIPort, proxycmd, stateDir,
	)
	if err != nil {
		return domain.ConnectionArtifacts{}, err
	}

	if c.VerifyAPIReady {
		tunnelPID, tunnelReused, err = c.verifyOrRecreateTunnel(
			target, cfg, hostname, username, keyfile, sshPort, localPort, proxycmd,
			internalIP, content, tunnelPID, tunnelReused, stateDir,
		)
		if err != nil {
			return domain.ConnectionArtifacts{}, err
		}
	}

	if _, err := c.mergeKubeconfig(content, target.ContextName()); err != nil {
		return domain.ConnectionArtifacts{}, err
	}

	_ = tunnelReused // used only internally; not surfaced in artifacts

	var argocdLocalPort *int
	var argocdLoginSuccess bool
	var argocdLoginMessage string
	if c.ArgocdAdapter != nil {
		argocdResult, argocdErr := c.ArgocdAdapter.Setup(
			target.ContextName(), domain.AutoDiscoverArgocdConfig(),
			hostname, username, keyfile, sshPort, proxycmd, internalIP,
		)
		if argocdErr == nil {
			argocdLocalPort = argocdResult.LocalPort
			argocdLoginSuccess = argocdResult.Success
			argocdLoginMessage = argocdResult.Message
		}
	}

	var alertmanagerLocalPort *int
	if c.AlertmanagerAdapter != nil {
		alertmanagerResult, alertmanagerErr := c.AlertmanagerAdapter.Setup(
			target.ContextName(), domain.AutoDiscoverAlertmanagerConfig(),
			hostname, username, keyfile, sshPort, proxycmd, internalIP,
		)
		if alertmanagerErr == nil {
			alertmanagerLocalPort = alertmanagerResult.LocalPort
		}
	}

	var victoriaMetricsLocalPort *int
	if c.VictoriaMetricsAdapter != nil {
		vmResult, vmErr := c.VictoriaMetricsAdapter.Setup(
			target.ContextName(), domain.AutoDiscoverVictoriaMetricsConfig(),
			hostname, username, keyfile, sshPort, proxycmd, internalIP,
		)
		if vmErr == nil {
			victoriaMetricsLocalPort = vmResult.LocalPort
		}
	}

	return domain.ConnectionArtifacts{
		LocalPort:                localPort,
		InternalIP:               internalIP,
		TunnelPID:                tunnelPID,
		UsedCache:                usedCache,
		ArgocdLocalPort:          argocdLocalPort,
		ArgocdLoginSuccess:       argocdLoginSuccess,
		ArgocdLoginMessage:       argocdLoginMessage,
		AlertmanagerLocalPort:    alertmanagerLocalPort,
		VictoriaMetricsLocalPort: victoriaMetricsLocalPort,
	}, nil
}

func (c *LocalClusterConnector) verifyOrRecreateTunnel(
	target domain.ClusterTarget,
	cfg domain.EffectiveConfig,
	hostname, username string,
	keyfile *string,
	sshPort, localPort int,
	proxycmd *string,
	internalIP, kubeconfigText string,
	tunnelPID *int,
	tunnelReused bool,
	stateDir string,
) (*int, bool, error) {
	err := c.pollAPIReady(localPort, kubeconfigText, c.APIReadyTimeout, c.APIReadyInterval)
	if err == nil {
		return tunnelPID, tunnelReused, nil
	}

	// Fresh tunnel failed: kill and raise immediately
	if !tunnelReused {
		c.killTunnel(target.ContextName(), stateDir)
		return nil, false, err
	}

	// Stale reused tunnel: kill and open a new one
	c.killTunnel(target.ContextName(), stateDir)
	newPID, newReused, ensErr := c.ensureTunnel(
		target.ContextName(), hostname, internalIP, username,
		keyfile, sshPort, localPort, cfg.K3sAPIPort, proxycmd, stateDir,
	)
	if ensErr != nil {
		return nil, false, ensErr
	}

	if err2 := c.pollAPIReady(localPort, kubeconfigText, c.APIReadyTimeout, c.APIReadyInterval); err2 != nil {
		if !newReused {
			c.killTunnel(target.ContextName(), stateDir)
		}
		return nil, false, err2
	}
	return newPID, newReused, nil
}
