package infrastructure

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/systemframe/k3ctx/internal/domain"
	"github.com/systemframe/k3ctx/internal/tunnel"
)

const (
	victoriaMetricsPortRangeStart = 48000
	victoriaMetricsPortRangeSize  = 10000
)

// LocalVictoriaMetricsConnector implements domain.VictoriaMetricsConnector using SSH tunnels.
type LocalVictoriaMetricsConnector struct {
	StateDir string

	// Injectable for testing:
	isTunnelRunning func(contextName, stateDir string) bool
	createTunnel    func(sshHost, internalIP string, local, remote int, opts tunnel.CreateTunnelOptions) (*int, error)
	saveTunnelPID   func(contextName string, pid *int, stateDir string)
	discoverService func(contextName string) *discoveredVictoriaMetricsService
}

type discoveredVictoriaMetricsService struct {
	Namespace string
	NodePort  int
}

// NewLocalVictoriaMetricsConnector returns a connector with real subprocess defaults.
func NewLocalVictoriaMetricsConnector() *LocalVictoriaMetricsConnector {
	c := &LocalVictoriaMetricsConnector{}
	c.isTunnelRunning = tunnel.IsTunnelRunning
	c.createTunnel = tunnel.CreateTunnel
	c.saveTunnelPID = tunnel.SaveTunnelPID
	c.discoverService = func(contextName string) *discoveredVictoriaMetricsService {
		return discoverVictoriaMetricsService(contextName, defaultKubectlRun)
	}
	return c
}

func (c *LocalVictoriaMetricsConnector) stateDir() string {
	if c.StateDir != "" {
		return c.StateDir
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "state", "k3ctx-tunnels")
}

// Setup implements domain.VictoriaMetricsConnector.
func (c *LocalVictoriaMetricsConnector) Setup(
	contextName string,
	cfg domain.VictoriaMetricsConfig,
	hostname, username string,
	keyfile *string,
	port int,
	proxycmd *string,
	internalIP string,
) (domain.VictoriaMetricsResult, error) {
	if !cfg.Enabled {
		return domain.VictoriaMetricsResult{
			Skipped: true,
			Message: "VictoriaMetrics not configured for this cluster",
		}, nil
	}
	if cfg.NodePort == nil && cfg.Discovery {
		if discovered := c.discoverService(contextName); discovered != nil {
			cfg.Namespace = discovered.Namespace
			cfg.NodePort = &discovered.NodePort
		}
	}
	if cfg.NodePort == nil {
		return domain.VictoriaMetricsResult{
			Skipped: true,
			Message: "VictoriaMetrics not discovered for this cluster",
		}, nil
	}

	vmContext := contextName + "-victoriametrics"
	localPort := tunnel.GetUniquePort(vmContext, victoriaMetricsPortRangeStart, victoriaMetricsPortRangeSize)
	stateDir := c.stateDir()

	if !c.isTunnelRunning(vmContext, stateDir) {
		opts := tunnel.CreateTunnelOptions{Username: username, Port: port}
		if keyfile != nil {
			opts.KeyFilename = *keyfile
		}
		if proxycmd != nil {
			opts.ProxyCmd = *proxycmd
		}
		pid, err := c.createTunnel(hostname, internalIP, localPort, *cfg.NodePort, opts)
		if err != nil {
			return domain.VictoriaMetricsResult{
				Message: fmt.Sprintf("victoriametrics tunnel failed: %v", err),
			}, nil
		}
		c.saveTunnelPID(vmContext, pid, stateDir)
	}

	return domain.VictoriaMetricsResult{
		LocalPort: &localPort,
	}, nil
}

func discoverVictoriaMetricsService(contextName string, kubectlRun func([]string) (string, error)) *discoveredVictoriaMetricsService {
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
		svcRank := victoriaMetricsServiceRank(item)
		if svcRank == 0 {
			continue
		}
		port, portRank, ok := bestVictoriaMetricsNodePort(item.Spec.Ports)
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
	return &discoveredVictoriaMetricsService{Namespace: best.item.Metadata.Namespace, NodePort: best.port.NodePort}
}

func victoriaMetricsServiceRank(item serviceItem) int {
	name := strings.ToLower(item.Metadata.Name)
	namespace := strings.ToLower(item.Metadata.Namespace)
	if item.Metadata.Labels["app.kubernetes.io/name"] == "victoria-metrics" {
		return 6
	}
	if item.Metadata.Labels["app"] == "vmsingle" {
		return 5
	}
	if name == "victoria-metrics" {
		return 4
	}
	if strings.HasPrefix(name, "vmsingle") {
		return 4
	}
	if namespace == "victoriametrics" || namespace == "vm" {
		return 3
	}
	if strings.Contains(name, "victoria") {
		return 2
	}
	if strings.Contains(namespace, "victoria") {
		return 1
	}
	return 0
}

func bestVictoriaMetricsNodePort(ports []servicePort) (servicePort, int, bool) {
	var best servicePort
	bestRank := 0
	for _, p := range ports {
		if p.NodePort == 0 {
			continue
		}
		rank := victoriaMetricsPortRank(p)
		if rank > bestRank {
			best = p
			bestRank = rank
		}
	}
	return best, bestRank, bestRank > 0
}

func victoriaMetricsPortRank(p servicePort) int {
	if p.Port == 8428 {
		return 5
	}
	if p.Port == 8481 {
		return 3
	}
	if strings.ToLower(p.Name) == "http" {
		return 2
	}
	return 1
}
