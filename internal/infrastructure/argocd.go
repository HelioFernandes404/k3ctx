package infrastructure

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
	discoverService func(contextName string) *discoveredArgocdService
	runArgocd       func(args []string, timeout time.Duration) error
}

type discoveredArgocdService struct {
	Namespace string
	NodePort  int
}

type serviceList struct {
	Items []serviceItem `json:"items"`
}

type serviceItem struct {
	Metadata serviceMetadata `json:"metadata"`
	Spec     serviceSpec     `json:"spec"`
}

type serviceMetadata struct {
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	Labels    map[string]string `json:"labels"`
}

type serviceSpec struct {
	Type  string        `json:"type"`
	Ports []servicePort `json:"ports"`
}

type servicePort struct {
	Name     string `json:"name"`
	Port     int    `json:"port"`
	NodePort int    `json:"nodePort"`
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
	c.discoverService = func(contextName string) *discoveredArgocdService {
		return discoverArgocdService(contextName, defaultKubectlRun)
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
	if !cfg.Enabled {
		return application.ArgocdLoginResult{
			Skipped: true,
			Message: "ArgoCD not configured for this cluster",
		}, nil
	}
	if cfg.NodePort == nil && cfg.Discovery {
		if discovered := c.discoverService(contextName); discovered != nil {
			cfg.Namespace = discovered.Namespace
			cfg.NodePort = &discovered.NodePort
		}
	}
	if cfg.NodePort == nil {
		return application.ArgocdLoginResult{
			Skipped: true,
			Message: "ArgoCD not discovered for this cluster",
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

func discoverArgocdService(contextName string, kubectlRun func([]string) (string, error)) *discoveredArgocdService {
	args := []string{"kubectl", "get", "svc", "-A", "-o", "json", "--context", contextName}
	out, err := kubectlRun(args)
	if err != nil {
		return nil
	}

	var services serviceList
	if err := json.Unmarshal([]byte(out), &services); err != nil {
		return nil
	}

	type candidate struct {
		item      serviceItem
		port      servicePort
		svcRank   int
		portRank  int
		itemIndex int
	}

	var candidates []candidate
	for i, item := range services.Items {
		svcRank := argocdServiceRank(item)
		if svcRank == 0 {
			continue
		}
		port, portRank, ok := bestNodePort(item.Spec.Ports)
		if !ok {
			continue
		}
		candidates = append(candidates, candidate{item: item, port: port, svcRank: svcRank, portRank: portRank, itemIndex: i})
	}
	if len(candidates) == 0 {
		return nil
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].svcRank != candidates[j].svcRank {
			return candidates[i].svcRank > candidates[j].svcRank
		}
		if candidates[i].portRank != candidates[j].portRank {
			return candidates[i].portRank > candidates[j].portRank
		}
		return candidates[i].itemIndex < candidates[j].itemIndex
	})

	best := candidates[0]
	return &discoveredArgocdService{Namespace: best.item.Metadata.Namespace, NodePort: best.port.NodePort}
}

func argocdServiceRank(item serviceItem) int {
	name := strings.ToLower(item.Metadata.Name)
	namespace := strings.ToLower(item.Metadata.Namespace)
	if item.Metadata.Labels["app.kubernetes.io/name"] == "argocd-server" {
		return 5
	}
	if name == "argocd-server" {
		return 4
	}
	if namespace == "argocd" {
		return 3
	}
	if strings.Contains(name, "argocd") {
		return 2
	}
	if strings.Contains(namespace, "argocd") {
		return 1
	}
	return 0
}

func bestNodePort(ports []servicePort) (servicePort, int, bool) {
	var best servicePort
	bestRank := 0
	for _, port := range ports {
		if port.NodePort == 0 {
			continue
		}
		rank := argocdPortRank(port)
		if rank > bestRank {
			best = port
			bestRank = rank
		}
	}
	return best, bestRank, bestRank > 0
}

func argocdPortRank(port servicePort) int {
	switch strings.ToLower(port.Name) {
	case "https":
		return 5
	case "http":
		return 3
	}
	switch port.Port {
	case 443:
		return 4
	case 80:
		return 2
	}
	return 1
}

func defaultKubectlRun(args []string) (string, error) {
	out, err := exec.Command(args[0], args[1:]...).Output() //nolint:gosec
	return strings.TrimSpace(string(out)), err
}
