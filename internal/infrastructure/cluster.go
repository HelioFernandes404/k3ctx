package infrastructure

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/systemframe/k3ctx/internal/application"
	"github.com/systemframe/k3ctx/internal/domain"
	"github.com/systemframe/k3ctx/internal/kubeconfig"
	"github.com/systemframe/k3ctx/internal/paths"
	sshpkg "github.com/systemframe/k3ctx/internal/ssh"
	"github.com/systemframe/k3ctx/internal/tunnel"
)

// LocalClusterConnector implements application.ClusterConnector using SSH tunnels.
type LocalClusterConnector struct {
	VerifyAPIReady             bool
	APIReadyTimeout            time.Duration
	APIReadyInterval           time.Duration
	StateDir                   string
	ArgocdAdapter              application.ArgocdConnector
	AlertmanagerAdapter        application.AlertmanagerConnector
	VictoriaMetricsAdapter     application.VictoriaMetricsConnector

	// Injectable for testing:
	sshConnect        func(target domain.ClusterTarget, cfg domain.EffectiveConfig) (hostname, username string, keyfile *string, sshPort int, proxycmd *string, runner func(string) (string, error), err error)
	prepareKubeconfig func(runner func(string) (string, error), target domain.ClusterTarget, cfg domain.EffectiveConfig) (internalIP string, localPort int, usedCache bool, content string, err error)
	ensureTunnel      func(contextName, hostname, internalIP, username string, keyfile *string, sshPort, localPort, k3sPort int, proxycmd *string, stateDir string) (pid *int, reused bool, err error)
	killTunnel        func(contextName, stateDir string)
	pollAPIReady      func(localPort int, kubeconfigText string, timeout, interval time.Duration) error
	mergeKubeconfig   func(content, contextName string) (string, error)
}

// NewLocalClusterConnector returns a connector with real subprocess defaults.
func NewLocalClusterConnector(argocd application.ArgocdConnector, alertmanager application.AlertmanagerConnector, victoriaMetrics application.VictoriaMetricsConnector) *LocalClusterConnector {
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
	c.sshConnect = defaultSSHConnect
	c.prepareKubeconfig = defaultPrepareKubeconfig
	c.ensureTunnel = defaultEnsureTunnel
	c.killTunnel = tunnel.KillTunnel
	c.pollAPIReady = defaultPollAPIReady
	c.mergeKubeconfig = func(content, ctx string) (string, error) {
		return kubeconfig.MergeKubeconfig(content, ctx)
	}
	return c
}

func (c *LocalClusterConnector) stateDir() string {
	if c.StateDir != "" {
		return c.StateDir
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "state", "k3ctx-tunnels")
}

// Connect implements application.ClusterConnector.
func (c *LocalClusterConnector) Connect(
	target domain.ClusterTarget,
	cfg domain.EffectiveConfig,
	_ domain.NetworkRequirement,
) (application.ConnectionArtifacts, error) {
	stateDir := c.stateDir()

	hostname, username, keyfile, sshPort, proxycmd, runner, err := c.sshConnect(target, cfg)
	if err != nil {
		return application.ConnectionArtifacts{}, err
	}

	internalIP, localPort, usedCache, content, err := c.prepareKubeconfig(runner, target, cfg)
	if err != nil {
		return application.ConnectionArtifacts{}, err
	}

	tunnelPID, tunnelReused, err := c.ensureTunnel(
		target.ContextName(), hostname, internalIP, username,
		keyfile, sshPort, localPort, cfg.K3sAPIPort, proxycmd, stateDir,
	)
	if err != nil {
		return application.ConnectionArtifacts{}, err
	}

	if c.VerifyAPIReady {
		tunnelPID, tunnelReused, err = c.verifyOrRecreateTunnel(
			target, cfg, hostname, username, keyfile, sshPort, localPort, proxycmd,
			internalIP, content, tunnelPID, tunnelReused, stateDir,
		)
		if err != nil {
			return application.ConnectionArtifacts{}, err
		}
	}

	if _, err := c.mergeKubeconfig(content, target.ContextName()); err != nil {
		return application.ConnectionArtifacts{}, err
	}

	_ = tunnelReused // used only internally; not surfaced in artifacts

	var argocdLocalPort *int
	if c.ArgocdAdapter != nil {
		argocdResult, argocdErr := c.ArgocdAdapter.Setup(
			target.ContextName(), domain.AutoDiscoverArgocdConfig(),
			hostname, username, keyfile, sshPort, proxycmd, internalIP,
		)
		if argocdErr == nil {
			argocdLocalPort = argocdResult.LocalPort
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

	return application.ConnectionArtifacts{
		LocalPort:                localPort,
		InternalIP:               internalIP,
		TunnelPID:                tunnelPID,
		UsedCache:                usedCache,
		ArgocdLocalPort:          argocdLocalPort,
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

// --- Default implementations ---

func defaultSSHConnect(target domain.ClusterTarget, cfg domain.EffectiveConfig) (string, string, *string, int, *string, func(string) (string, error), error) {
	sshCfg := sshpkg.LoadSSHConfig(target.HostAlias(), cfg.SSHConfigPath)
	hostname, username, keyfile, sshPort, proxycmd, err := sshpkg.ResolveConnectionTarget(
		target.HostAlias(), sshCfg, cfg.SSHKeyPath, target.HostConfig(),
	)
	if err != nil {
		return "", "", nil, 0, nil, nil, err
	}
	runner := sshpkg.MakeRemoteRunner(hostname, username, keyfile, sshPort, proxycmd)
	return hostname, username, keyfile, sshPort, proxycmd, runner, nil
}

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
	return pid, false, nil
}

func readTunnelPID(contextName, stateDir string) *int {
	pidFile := tunnel.GetTunnelPIDFile(contextName, stateDir)
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return nil
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return nil
	}
	return &pid
}

// --- API readiness polling ---

type httpDoer func(url string, perReqTimeout time.Duration, tlsConfig *tls.Config) (int, error)

func defaultPollAPIReady(localPort int, kubeconfigText string, timeout, interval time.Duration) error {
	tlsConfig := buildTLSConfig(kubeconfigText)
	return pollAPIReadyWithDoer(localPort, timeout, interval, tlsConfig, defaultHTTPDoer)
}

func pollAPIReadyWithDoer(localPort int, timeout, interval time.Duration, tlsConfig *tls.Config, doer httpDoer) error {
	url := fmt.Sprintf("https://127.0.0.1:%d/version", localPort)
	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		perReq := interval
		if perReq < 2*time.Second {
			perReq = 2 * time.Second
		}
		if perReq > remaining {
			perReq = remaining
		}
		if perReq <= 0 {
			break
		}

		code, err := doer(url, perReq, tlsConfig)
		if err == nil && code >= 200 && code < 500 {
			return nil
		}
		lastErr = err

		sleep := interval
		if rem := time.Until(deadline); sleep > rem {
			sleep = rem
		}
		if sleep > 0 {
			time.Sleep(sleep)
		}
	}

	detail := ""
	if lastErr != nil {
		detail = lastErr.Error()
	}
	return &application.ClusterConnectionError{
		Code:      "kubernetes_api_unreachable",
		Message:   fmt.Sprintf("Kubernetes API did not become ready on %s", url),
		Detail:    detail,
		Retryable: true,
	}
}

func defaultHTTPDoer(url string, timeout time.Duration, tlsConfig *tls.Config) (int, error) {
	client := &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
	}
	resp, err := client.Get(url) //nolint:noctx
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

// buildTLSConfig extracts TLS material from a kubeconfig YAML and creates a tls.Config.
// Falls back to InsecureSkipVerify when the kubeconfig has no usable certs.
func buildTLSConfig(kubeconfigText string) *tls.Config {
	tlsCfg := &tls.Config{InsecureSkipVerify: true} //nolint:gosec

	var kube map[string]any
	if err := yaml.Unmarshal([]byte(kubeconfigText), &kube); err != nil {
		return tlsCfg
	}

	clusters, _ := kube["clusters"].([]any)
	if len(clusters) > 0 {
		cluster, _ := clusters[0].(map[string]any)
		clusterData, _ := cluster["cluster"].(map[string]any)
		if caData, ok := clusterData["certificate-authority-data"].(string); ok && caData != "" {
			caDER, err := base64.StdEncoding.DecodeString(caData)
			if err == nil {
				pool := x509.NewCertPool()
				pool.AppendCertsFromPEM(caDER)
				tlsCfg.RootCAs = pool
				tlsCfg.InsecureSkipVerify = false
			}
		}
	}

	users, _ := kube["users"].([]any)
	if len(users) > 0 {
		user, _ := users[0].(map[string]any)
		userData, _ := user["user"].(map[string]any)
		certData, hasCert := userData["client-certificate-data"].(string)
		keyData, hasKey := userData["client-key-data"].(string)
		if hasCert && hasKey {
			certPEM, err1 := base64.StdEncoding.DecodeString(certData)
			keyPEM, err2 := base64.StdEncoding.DecodeString(keyData)
			if err1 == nil && err2 == nil {
				if pair, err := tls.X509KeyPair(certPEM, keyPEM); err == nil {
					tlsCfg.Certificates = []tls.Certificate{pair}
				}
			}
		}
	}

	return tlsCfg
}
