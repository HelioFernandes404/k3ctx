package infrastructure

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/systemframe/k3ctx/internal/domain"
	"github.com/systemframe/k3ctx/internal/netbird"
)

const defaultNetBirdGroup = "k3s_cluster"

// NetBirdInventoryCatalog discovers cluster targets from the NetBird peer list.
// StatusFn is injectable for testing; when nil, the catalog calls netbird up then RunStatus(BinPath).
type NetBirdInventoryCatalog struct {
	BinPath    string
	HostFilter string
	StatusFn   func() ([]netbird.Peer, error)
}

func (c NetBirdInventoryCatalog) ListTargets() ([]domain.ClusterTarget, error) {
	peers, err := c.fetchPeers()
	if err != nil {
		return nil, err
	}

	var filter *regexp.Regexp
	if c.HostFilter != "" {
		filter, err = regexp.Compile(c.HostFilter)
		if err != nil {
			return nil, fmt.Errorf("invalid netbird_host_filter %q: %w", c.HostFilter, err)
		}
	}

	var targets []domain.ClusterTarget
	for _, p := range peers {
		parts := strings.SplitN(p.FQDN, ".", 3)
		if len(parts) < 2 {
			continue
		}
		hostAlias := parts[0]
		client := parts[1]

		if filter != nil && !filter.MatchString(hostAlias) {
			continue
		}

		hostConfig := map[string]any{
			"addr":           p.FQDN,
			"netbird_status": p.Status,
			"netbird_ip":     p.IP,
		}
		targets = append(targets, domain.NewClusterTarget(client, hostAlias, defaultNetBirdGroup, hostConfig, nil))
	}
	return targets, nil
}

func (c NetBirdInventoryCatalog) fetchPeers() ([]netbird.Peer, error) {
	if c.StatusFn != nil {
		return c.StatusFn()
	}
	binPath := c.BinPath
	if binPath == "" {
		binPath = "netbird"
	}
	// Ensure the daemon is connected before querying peers.
	_ = exec.Command(binPath, "up").Run()
	data, err := netbird.RunStatus(binPath)
	if err != nil {
		return nil, err
	}
	return netbird.ParsePeers(data)
}
