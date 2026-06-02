package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systemframe/k3ctx/internal/domain"
)

func TestRunHosts_All_ReturnsAllItems(t *testing.T) {
	jsonOutput = true
	hostsAll = true
	t.Cleanup(func() { jsonOutput = false; hostsAll = false })

	svcs.Catalog = &mockCatalog{targets: []domain.ClusterTarget{
		domain.NewClusterTarget("acme", "prod1", "k3s", nil, nil),
		domain.NewClusterTarget("acme", "prod2", "k3s", nil, nil),
		domain.NewClusterTarget("acme", "prod3", "k3s", nil, nil),
		domain.NewClusterTarget("other", "prod1", "k3s", nil, nil),
	}}

	var buf bytes.Buffer
	hostsCmd.SetOut(&buf)
	t.Cleanup(func() { hostsCmd.SetOut(nil) })

	err := runHosts(hostsCmd, []string{"acme"})
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	items := got["items"].([]any)
	assert.Len(t, items, 3, "should return only acme hosts")
	_, hasPage := got["page"]
	assert.False(t, hasPage, "all mode should omit pagination metadata")
}
