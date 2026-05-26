package netbird_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systemframe/k3ctx/internal/netbird"
)

var fixtureJSON = []byte(`{
  "peers": {
    "total": 3,
    "connected": 2,
    "details": [
      {
        "fqdn": "sf-prd-us-00001.systemframe.vpn",
        "netbirdIp": "100.64.32.127",
        "status": "Connected"
      },
      {
        "fqdn": "sf-tst-sp-00001.systemframe.vpn",
        "netbirdIp": "100.64.108.87",
        "status": "Connecting"
      },
      {
        "fqdn": "paulao-laptop.systemframe.vpn",
        "netbirdIp": "100.64.20.38",
        "status": "Connected"
      }
    ]
  }
}`)

func TestParsePeers_normal(t *testing.T) {
	peers, err := netbird.ParsePeers(fixtureJSON)
	require.NoError(t, err)
	assert.Len(t, peers, 3)

	assert.Equal(t, "sf-prd-us-00001.systemframe.vpn", peers[0].FQDN)
	assert.Equal(t, "100.64.32.127", peers[0].IP)
	assert.Equal(t, "Connected", peers[0].Status)

	assert.Equal(t, "sf-tst-sp-00001.systemframe.vpn", peers[1].FQDN)
	assert.Equal(t, "Connecting", peers[1].Status)
}

func TestParsePeers_empty(t *testing.T) {
	peers, err := netbird.ParsePeers([]byte(`{"peers":{"details":[]}}`))
	require.NoError(t, err)
	assert.Empty(t, peers)
}

func TestParsePeers_malformed(t *testing.T) {
	_, err := netbird.ParsePeers([]byte(`not json`))
	assert.Error(t, err)
}

func TestParsePeers_missingPeersSection(t *testing.T) {
	peers, err := netbird.ParsePeers([]byte(`{}`))
	require.NoError(t, err)
	assert.Empty(t, peers)
}

func TestRunStatus_invalidBin(t *testing.T) {
	_, err := netbird.RunStatus("/nonexistent/netbird-bin")
	assert.ErrorContains(t, err, "netbird CLI unavailable")
}
