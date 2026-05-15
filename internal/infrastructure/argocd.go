package infrastructure

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/systemframe/k3ctx/internal/application"
	"github.com/systemframe/k3ctx/internal/domain"
	"github.com/systemframe/k3ctx/internal/tunnel"
)

const (
	argocdPortRangeStart = 28000
	argocdPortRangeSize  = 10000
)

// LocalArgocdConnector implements application.ArgocdConnector using SSH tunnels
// and the argocd CLI for login.
type LocalArgocdConnector struct {
	StateDir string

	// Injectable for testing:
	isTunnelRunning func(contextName, stateDir string) bool
	createTunnel    func(sshHost, internalIP string, local, remote int, opts tunnel.CreateTunnelOptions) (*int, error)
	saveTunnelPID   func(contextName string, pid *int, stateDir string)
	which           func(name string) string
	fetchPassword   func(contextName, namespace string) *string
	runArgocd       func(args []string, timeout time.Duration) error
}

// NewLocalArgocdConnector returns a connector with real subprocess defaults.
func NewLocalArgocdConnector() *LocalArgocdConnector {
	c := &LocalArgocdConnector{}
	c.isTunnelRunning = tunnel.IsTunnelRunning
	c.createTunnel = tunnel.CreateTunnel
	c.saveTunnelPID = tunnel.SaveTunnelPID
	c.which = func(name string) string {
		p, _ := exec.LookPath(name)
		return p
	}
	c.fetchPassword = func(contextName, namespace string) *string {
		return fetchArgocdPassword(contextName, namespace, defaultKubectlRun)
	}
	c.runArgocd = func(args []string, timeout time.Duration) error {
		cmd := exec.Command("argocd", args...) //nolint:gosec
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	return c
}

func (c *LocalArgocdConnector) stateDir() string {
	if c.StateDir != "" {
		return c.StateDir
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "state", "k9s-tunnels")
}

// Setup implements application.ArgocdConnector.
func (c *LocalArgocdConnector) Setup(
	contextName string,
	cfg domain.ArgocdConfig,
	hostname, username string,
	keyfile *string,
	port int,
	proxycmd *string,
	internalIP string,
) (application.ArgocdLoginResult, error) {
	if !cfg.Enabled || cfg.NodePort == nil {
		return application.ArgocdLoginResult{
			Skipped: true,
			Message: "ArgoCD not configured for this cluster",
		}, nil
	}

	argocdContext := contextName + "-argocd"
	localPort := tunnel.GetUniquePort(argocdContext, argocdPortRangeStart, argocdPortRangeSize)
	stateDir := c.stateDir()

	if !c.isTunnelRunning(argocdContext, stateDir) {
		opts := tunnel.CreateTunnelOptions{Username: username, Port: port}
		if keyfile != nil {
			opts.KeyFilename = *keyfile
		}
		if proxycmd != nil {
			opts.ProxyCmd = *proxycmd
		}
		pid, err := c.createTunnel(hostname, internalIP, localPort, *cfg.NodePort, opts)
		if err != nil {
			return application.ArgocdLoginResult{}, fmt.Errorf("argocd tunnel: %w", err)
		}
		c.saveTunnelPID(argocdContext, pid, stateDir)
	}

	if c.which("argocd") == "" {
		return application.ArgocdLoginResult{
			LocalPort: &localPort,
			Message:   fmt.Sprintf("argocd CLI not found in PATH; tunnel open at 127.0.0.1:%d", localPort),
		}, nil
	}

	password := c.fetchPassword(contextName, cfg.Namespace)
	if password == nil {
		return application.ArgocdLoginResult{
			LocalPort: &localPort,
			Message:   fmt.Sprintf("argocd-initial-admin-secret not found in namespace %q; run: argocd login --insecure 127.0.0.1:%d", cfg.Namespace, localPort),
		}, nil
	}

	tlsFlag := "--insecure"
	if cfg.Plaintext {
		tlsFlag = "--plaintext"
	}
	loginArgs := []string{
		"login", tlsFlag,
		"--username", "admin",
		"--password", *password,
		fmt.Sprintf("127.0.0.1:%d", localPort),
	}
	if err := c.runArgocd(loginArgs, 30*time.Second); err != nil {
		return application.ArgocdLoginResult{
			LocalPort: &localPort,
			Message:   fmt.Sprintf("argocd login failed: %v", err),
		}, nil
	}

	return application.ArgocdLoginResult{
		Success:   true,
		LocalPort: &localPort,
	}, nil
}

// fetchArgocdPassword fetches and base64-decodes the argocd admin password via kubectl.
// kubectlRun is injectable for testing.
func fetchArgocdPassword(contextName, namespace string, kubectlRun func([]string) (string, error)) *string {
	args := []string{
		"kubectl", "get", "secret", "argocd-initial-admin-secret",
		"-n", namespace,
		"-o", "jsonpath={.data.password}",
		"--context", contextName,
	}
	out, err := kubectlRun(args)
	if err != nil {
		return nil
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return nil
	}
	data, err := base64.StdEncoding.DecodeString(out)
	if err != nil {
		return nil
	}
	s := string(data)
	return &s
}

func defaultKubectlRun(args []string) (string, error) {
	out, err := exec.Command(args[0], args[1:]...).Output() //nolint:gosec
	return strings.TrimSpace(string(out)), err
}
