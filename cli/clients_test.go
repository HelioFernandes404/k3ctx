package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systemframe/k3ctx/internal/domain"
)

func TestRunClients_All_ReturnsAllItems(t *testing.T) {
	jsonOutput = true
	clientsAll = true
	t.Cleanup(func() { jsonOutput = false; clientsAll = false })

	svcs.Catalog = &mockCatalog{targets: []domain.ClusterTarget{
		domain.NewClusterTarget("acme", "prod", "k3s", nil, nil),
		domain.NewClusterTarget("beta", "prod", "k3s", nil, nil),
		domain.NewClusterTarget("gamma", "prod", "k3s", nil, nil),
	}}

	var buf bytes.Buffer
	clientsCmd.SetOut(&buf)
	t.Cleanup(func() { clientsCmd.SetOut(nil) })

	err := runClients(clientsCmd, nil)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	assert.Equal(t, true, got["ok"])
	data := got["data"].(map[string]any)
	items := data["items"].([]any)
	assert.Len(t, items, 3)
	_, hasPage := data["page"]
	assert.False(t, hasPage, "all mode should omit pagination metadata")
}

func TestRunClients_All_EmptyWhenNoTargets(t *testing.T) {
	jsonOutput = true
	clientsAll = true
	t.Cleanup(func() { jsonOutput = false; clientsAll = false })

	svcs.Catalog = &mockCatalog{targets: nil}

	var buf bytes.Buffer
	clientsCmd.SetOut(&buf)
	t.Cleanup(func() { clientsCmd.SetOut(nil) })

	err := runClients(clientsCmd, nil)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	data := got["data"].(map[string]any)
	items := data["items"].([]any)
	assert.Empty(t, items)
}
