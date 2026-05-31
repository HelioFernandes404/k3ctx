package infrastructure_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systemframe/k3ctx/internal/infrastructure"
	"github.com/systemframe/k3ctx/internal/netbird"
)

func buildCatalog(filter string, peers []netbird.Peer) infrastructure.NetBirdInventoryCatalog {
	return infrastructure.NetBirdInventoryCatalog{
		HostFilter: filter,
		StatusFn:   func() ([]netbird.Peer, error) { return peers, nil },
	}
}

func TestNetBirdCatalog_normal(t *testing.T) {
	peers := []netbird.Peer{
		{FQDN: "sf-prd-us-00001.systemframe.vpn", IP: "100.64.32.127", Status: "Connected"},
		{FQDN: "sf-tst-sp-00003.systemframe.vpn", IP: "100.64.131.168", Status: "Connecting"},
	}
	cat := buildCatalog(`^sf-[a-z]{3}-(?:[a-z]{2}|us)-[0-9]{5}`, peers)
	targets, err := cat.ListTargets()
	require.NoError(t, err)
	require.Len(t, targets, 2)

	assert.Equal(t, "sf-prd-us-00001", targets[0].HostAlias())
	assert.Equal(t, "systemframe", targets[0].Company())
	assert.Equal(t, "k3s_cluster", targets[0].Group())
	assert.Equal(t, "sf-prd-us-00001.systemframe.vpn", targets[0].HostConfig()["addr"])
	assert.Equal(t, "Connected", targets[0].HostConfig()["netbird_status"])
}

func TestNetBirdCatalog_fqdnSkip(t *testing.T) {
	peers := []netbird.Peer{
		{FQDN: "nolabels", IP: "100.64.1.1", Status: "Connected"},
		{FQDN: "sf-prd-us-00001.systemframe.vpn", IP: "100.64.32.127", Status: "Connected"},
	}
	cat := buildCatalog(`^sf-[a-z]{3}-(?:[a-z]{2}|us)-[0-9]{5}`, peers)
	targets, err := cat.ListTargets()
	require.NoError(t, err)
	assert.Len(t, targets, 1)
	assert.Equal(t, "sf-prd-us-00001", targets[0].HostAlias())
}

func TestNetBirdCatalog_filterExcludesPersonal(t *testing.T) {
	peers := []netbird.Peer{
		{FQDN: "sf-prd-us-00001.systemframe.vpn", IP: "100.64.32.127", Status: "Connected"},
		{FQDN: "paulao-laptop.systemframe.vpn", IP: "100.64.20.38", Status: "Connected"},
		{FQDN: "ipad-paulo.systemframe.vpn", IP: "100.64.213.93", Status: "Connecting"},
	}
	cat := buildCatalog(`^sf-[a-z]{3}-(?:[a-z]{2}|us)-[0-9]{5}`, peers)
	targets, err := cat.ListTargets()
	require.NoError(t, err)
	assert.Len(t, targets, 1)
}

func TestNetBirdCatalog_noFilter(t *testing.T) {
	peers := []netbird.Peer{
		{FQDN: "sf-prd-us-00001.systemframe.vpn", IP: "100.64.32.127", Status: "Connected"},
		{FQDN: "paulao-laptop.systemframe.vpn", IP: "100.64.20.38", Status: "Connected"},
	}
	cat := buildCatalog("", peers)
	targets, err := cat.ListTargets()
	require.NoError(t, err)
	assert.Len(t, targets, 2)
}

func TestNetBirdCatalog_cliError(t *testing.T) {
	cat := infrastructure.NetBirdInventoryCatalog{
		HostFilter: "",
		StatusFn:   func() ([]netbird.Peer, error) { return nil, assert.AnError },
	}
	_, err := cat.ListTargets()
	assert.Error(t, err)
}

func TestNetBirdCatalog_contextName(t *testing.T) {
	peers := []netbird.Peer{
		{FQDN: "sf-prd-us-00001.systemframe.vpn", IP: "100.64.32.127", Status: "Connected"},
	}
	cat := buildCatalog("", peers)
	targets, err := cat.ListTargets()
	require.NoError(t, err)
	require.Len(t, targets, 1)
	assert.Equal(t, "systemframe-sf-prd-us-00001", targets[0].ContextName())
}

func TestNetBirdCatalog_offlinePeerIncluded(t *testing.T) {
	peers := []netbird.Peer{
		{FQDN: "sf-prd-us-00001.systemframe.vpn", IP: "100.64.32.127", Status: "Connected"},
		{FQDN: "sf-tst-sp-00001.systemframe.vpn", IP: "100.64.108.87", Status: "Connecting"},
	}
	cat := buildCatalog(`^sf-[a-z]{3}-(?:[a-z]{2}|us)-[0-9]{5}`, peers)
	targets, err := cat.ListTargets()
	require.NoError(t, err)
	assert.Len(t, targets, 2)

	found := false
	for _, t2 := range targets {
		if t2.HostAlias() == "sf-tst-sp-00001" {
			assert.Equal(t, "Connecting", t2.HostConfig()["netbird_status"])
			found = true
		}
	}
	assert.True(t, found, "offline peer should be included")
}
