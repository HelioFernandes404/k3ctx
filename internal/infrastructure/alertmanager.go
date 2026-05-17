package infrastructure

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/systemframe/k3ctx/internal/application"
	"github.com/systemframe/k3ctx/internal/domain"
	"github.com/systemframe/k3ctx/internal/tunnel"
)

const (
	alertmanagerPortRangeStart = 38000
	alertmanagerPortRangeSize  = 10000
)

// LocalAlertmanagerConnector implements application.AlertmanagerConnector using SSH tunnels.
type LocalAlertmanagerConnector struct {
	StateDir string

	// Injectable for testing:
	isTunnelRunning func(contextName, stateDir string) bool
	createTunnel    func(sshHost, internalIP string, local, remote int, opts tunnel.CreateTunnelOptions) (*int, error)
	saveTunnelPID   func(contextName string, pid *int, stateDir string)
	discoverService func(contextName string) *discoveredAlertmanagerService
}

type discoveredAlertmanagerService struct {
	Namespace string
	NodePort  int
}

// NewLocalAlertmanagerConnector returns a connector with real subprocess defaults.
func NewLocalAlertmanagerConnector() *LocalAlertmanagerConnector {
	c := &LocalAlertmanagerConnector{}
	c.isTunnelRunning = tunnel.IsTunnelRunning
	c.createTunnel = tunnel.CreateTunnel
	c.saveTunnelPID = tunnel.SaveTunnelPID
	c.discoverService = func(contextName string) *discoveredAlertmanagerService {
		return discoverAlertmanagerService(contextName, defaultKubectlRun)
	}
	return c
}

func (c *LocalAlertmanagerConnector) stateDir() string {
	if c.StateDir != "" {
		return c.StateDir
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "state", "k3ctx-tunnels")
}

// Setup implements application.AlertmanagerConnector.
func (c *LocalAlertmanagerConnector) Setup(
	contextName string,
	cfg domain.AlertmanagerConfig,
	hostname, username string,
	keyfile *string,
	port int,
	proxycmd *string,
	internalIP string,
) (application.AlertmanagerResult, error) {
	if !cfg.Enabled {
		return application.AlertmanagerResult{
			Skipped: true,
			Message: "Alertmanager not configured for this cluster",
		}, nil
	}
	if cfg.NodePort == nil && cfg.Discovery {
		if discovered := c.discoverService(contextName); discovered != nil {
			cfg.Namespace = discovered.Namespace
			cfg.NodePort = &discovered.NodePort
		}
	}
	if cfg.NodePort == nil {
		return application.AlertmanagerResult{
			Skipped: true,
			Message: "Alertmanager not discovered for this cluster",
		}, nil
	}

	alertmanagerContext := contextName + "-alertmanager"
	localPort := tunnel.GetUniquePort(alertmanagerContext, alertmanagerPortRangeStart, alertmanagerPortRangeSize)
	stateDir := c.stateDir()

	if !c.isTunnelRunning(alertmanagerContext, stateDir) {
		opts := tunnel.CreateTunnelOptions{Username: username, Port: port}
		if keyfile != nil {
			opts.KeyFilename = *keyfile
		}
		if proxycmd != nil {
			opts.ProxyCmd = *proxycmd
		}
		pid, err := c.createTunnel(hostname, internalIP, localPort, *cfg.NodePort, opts)
		if err != nil {
			return application.AlertmanagerResult{
				Message: fmt.Sprintf("alertmanager tunnel failed: %v", err),
			}, nil
		}
		c.saveTunnelPID(alertmanagerContext, pid, stateDir)
	}

	return application.AlertmanagerResult{
		LocalPort: &localPort,
	}, nil
}

func discoverAlertmanagerService(contextName string, kubectlRun func([]string) (string, error)) *discoveredAlertmanagerService {
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
		svcRank := alertmanagerServiceRank(item)
		if svcRank == 0 {
			continue
		}
		port, portRank, ok := bestAlertmanagerNodePort(item.Spec.Ports)
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
	return &discoveredAlertmanagerService{Namespace: best.item.Metadata.Namespace, NodePort: best.port.NodePort}
}

func alertmanagerServiceRank(item serviceItem) int {
	name := strings.ToLower(item.Metadata.Name)
	namespace := strings.ToLower(item.Metadata.Namespace)
	if item.Metadata.Labels["app.kubernetes.io/name"] == "alertmanager" {
		return 6
	}
	if name == "alertmanager" && namespace == "monitoring" {
		return 5
	}
	if name == "alertmanager" {
		return 4
	}
	if namespace == "monitoring" && strings.Contains(name, "alertmanager") {
		return 3
	}
	if strings.Contains(name, "alertmanager") {
		return 2
	}
	if strings.Contains(namespace, "alertmanager") {
		return 1
	}
	return 0
}

func bestAlertmanagerNodePort(ports []servicePort) (servicePort, int, bool) {
	var best servicePort
	bestRank := 0
	for _, p := range ports {
		if p.NodePort == 0 {
			continue
		}
		rank := alertmanagerPortRank(p)
		if rank > bestRank {
			best = p
			bestRank = rank
		}
	}
	return best, bestRank, bestRank > 0
}

func alertmanagerPortRank(p servicePort) int {
	if p.Port == 9093 {
		return 5
	}
	if strings.ToLower(p.Name) == "http" {
		return 3
	}
	return 1
}
