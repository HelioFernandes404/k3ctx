package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systemframe/k3ctx/internal/application"
	"github.com/systemframe/k3ctx/internal/domain"
)

func TestRunConnect_NoMatch(t *testing.T) {
	// Empty catalog → ResolveHost returns ResolutionNoMatch
	svcs.Catalog = &mockCatalog{targets: []domain.ClusterTarget{}}

	err := runConnect(connectCmd, []string{"nonexistent-host"})

	require.Error(t, err)
	var exitErr *ExitErr
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 4, exitErr.Code)
	assert.Equal(t, "NO_MATCH", exitErr.ECode)
	assert.NotEmpty(t, exitErr.Msg)
}

func TestRunConnect_AmbiguousMatch(t *testing.T) {
	// Two targets with the same query match → AMBIGUOUS_MATCH
	t1 := domain.NewClusterTarget("acme", "prod1", "k3s", map[string]any{"addr": "10.0.0.1"}, nil)
	t2 := domain.NewClusterTarget("acme", "prod2", "k3s", map[string]any{"addr": "10.0.0.2"}, nil)
	svcs.Catalog = &mockCatalog{targets: []domain.ClusterTarget{t1, t2}}

	// The query "acme" matches both targets (substring match on client/host)
	err := runConnect(connectCmd, []string{"acme"})

	require.Error(t, err)
	var exitErr *ExitErr
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 3, exitErr.Code)
	assert.Equal(t, "AMBIGUOUS_MATCH", exitErr.ECode)
	assert.Contains(t, exitErr.Msg, "Multiple hosts matched")
}

func TestRunConnect_CatalogError(t *testing.T) {
	svcs.Catalog = &mockCatalog{err: errors.New("inventory read failed")}

	err := runConnect(connectCmd, []string{"any"})

	require.Error(t, err)
	// Should propagate as a plain error, not ExitErr
	var exitErr *ExitErr
	assert.False(t, errors.As(err, &exitErr), "catalog errors should not become ExitErr")
}

// --- --all-hosts ---

// --- --dry-run ---

func TestRunConnect_DryRun_ReturnsResolutionWithoutConnecting(t *testing.T) {
	svcs.Catalog = &mockCatalog{targets: []domain.ClusterTarget{
		domain.NewClusterTarget("acme", "prod", "k3s", map[string]any{"addr": "10.0.0.1"}, nil),
	}}
	// If connector were called it would return an error — dry-run must skip it.
	svcs.Connector = &mockConnector{err: errors.New("should not be called")}
	connectDryRun = true
	jsonOutput = true
	t.Cleanup(func() { connectDryRun = false; jsonOutput = false; svcs.Connector = nil })

	var buf bytes.Buffer
	connectCmd.SetOut(&buf)
	t.Cleanup(func() { connectCmd.SetOut(nil) })

	err := runConnect(connectCmd, []string{"acme-prod"})
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	assert.Equal(t, true, got["dry_run"])
	assert.Equal(t, "acme-prod", got["context_name"])
	assert.Equal(t, "acme", got["client"])
	assert.Equal(t, "prod", got["host"])
}

func TestRunConnect_DryRun_NoMatch_ReturnsError(t *testing.T) {
	svcs.Catalog = &mockCatalog{targets: []domain.ClusterTarget{}}
	connectDryRun = true
	t.Cleanup(func() { connectDryRun = false })

	err := runConnect(connectCmd, []string{"nonexistent"})

	require.Error(t, err)
	var exitErr *ExitErr
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, "NO_MATCH", exitErr.ECode)
}

func TestBuildConnectQuery_TwoArgs_SetsClientAndHostName(t *testing.T) {
	q := buildConnectQuery([]string{"systemframe", "sf-tst-sp-00003"})
	require.NotNil(t, q.Client)
	assert.Equal(t, "systemframe", *q.Client)
	require.NotNil(t, q.HostName)
	assert.Equal(t, "sf-tst-sp-00003", *q.HostName)
	assert.Nil(t, q.Query)
}

func TestBuildConnectQuery_OneArg_SetsQuery(t *testing.T) {
	q := buildConnectQuery([]string{"sf-tst-sp-00003"})
	require.NotNil(t, q.Query)
	assert.Equal(t, "sf-tst-sp-00003", *q.Query)
	assert.Nil(t, q.Client)
	assert.Nil(t, q.HostName)
}

func TestRunConnect_TwoArgs_ResolvesSpecificHostBySfID(t *testing.T) {
	t1 := domain.NewClusterTarget("systemframe", "sf-prd-us-00002", "k3s", map[string]any{"addr": "sf-prd-us-00002.systemframe.vpn"}, nil)
	t2 := domain.NewClusterTarget("systemframe", "sf-tst-sp-00003", "k3s", map[string]any{"addr": "sf-tst-sp-00003.systemframe.vpn"}, nil)
	svcs.Catalog = &mockCatalog{targets: []domain.ClusterTarget{t1, t2}}
	svcs.Connector = &mockConnector{err: nil}
	connectDryRun = true
	jsonOutput = true
	t.Cleanup(func() { connectDryRun = false; jsonOutput = false; svcs.Connector = nil })

	var buf bytes.Buffer
	connectCmd.SetOut(&buf)
	t.Cleanup(func() { connectCmd.SetOut(nil) })

	err := runConnect(connectCmd, []string{"systemframe", "sf-tst-sp-00003"})
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	assert.Equal(t, "systemframe-sf-tst-sp-00003", got["context_name"])
}

func TestRunConnect_AllHosts_NoMatch(t *testing.T) {
	svcs.Catalog = &mockCatalog{targets: []domain.ClusterTarget{
		domain.NewClusterTarget("other", "host1", "k3s", map[string]any{"addr": "10.0.0.1"}, nil),
	}}
	connectAllHosts = "acme"
	t.Cleanup(func() { connectAllHosts = "" })

	err := runConnect(connectCmd, nil)

	require.Error(t, err)
	var exitErr *ExitErr
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 4, exitErr.Code)
	assert.Equal(t, "NO_MATCH", exitErr.ECode)
}

func TestRunConnect_AllHosts_MutualExclusionWithArgs(t *testing.T) {
	connectAllHosts = "acme"
	t.Cleanup(func() { connectAllHosts = "" })

	err := runConnect(connectCmd, []string{"extra-arg"})

	require.Error(t, err)
	var exitErr *ExitErr
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, "USAGE_ERROR", exitErr.ECode)
}

func TestRunConnect_AllHosts_MultiResultJSON(t *testing.T) {
	t1 := domain.NewClusterTarget("acme", "prod1", "k3s", map[string]any{"addr": "10.0.0.1"}, nil)
	t2 := domain.NewClusterTarget("acme", "prod2", "k3s", map[string]any{"addr": "10.0.0.2"}, nil)
	svcs.Catalog = &mockCatalog{targets: []domain.ClusterTarget{t1, t2}}
	svcs.Connector = &mockConnector{artifacts: application.ConnectionArtifacts{
		LocalPort:  16500,
		InternalIP: "10.0.0.1",
	}}
	connectAllHosts = "acme"
	jsonOutput = true
	t.Cleanup(func() {
		connectAllHosts = ""
		jsonOutput = false
		svcs.Connector = nil
	})

	var buf bytes.Buffer
	connectCmd.SetOut(&buf)
	t.Cleanup(func() { connectCmd.SetOut(nil) })

	err := runConnect(connectCmd, nil)
	require.NoError(t, err)

	var data []any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &data))
	assert.Len(t, data, 2)
}
