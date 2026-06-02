package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systemframe/k3ctx/internal/domain"
)

func TestRunExec_NoArgsReturnsUsageError(t *testing.T) {
	err := runExec(execCmd, nil)
	require.Error(t, err)
	var exitErr *ExitErr
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, "USAGE_ERROR", exitErr.ECode)
}

func TestRunExec_NoLiveContexts_JSON_EmitsEmptyArray(t *testing.T) {
	jsonOutput = true
	t.Cleanup(func() { jsonOutput = false })

	svcs.Status = &mockStatusReader{items: []map[string]any{
		{"context_name": "acme-prod", "liveness": "dead"},
	}}

	var buf bytes.Buffer
	execCmd.SetOut(&buf)
	t.Cleanup(func() { execCmd.SetOut(nil) })

	err := runExec(execCmd, []string{"get", "nodes"})
	require.NoError(t, err)

	var got []any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	assert.Empty(t, got)
}

func TestRunExec_NoLiveContexts_Text_PrintsMessage(t *testing.T) {
	jsonOutput = false
	svcs.Status = &mockStatusReader{items: []map[string]any{}}

	var buf bytes.Buffer
	execCmd.SetOut(&buf)
	t.Cleanup(func() { execCmd.SetOut(nil) })

	err := runExec(execCmd, []string{"get", "nodes"})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No live tunnels")
}

func TestRunExec_LiveContexts_JSON_IncludesAllResults(t *testing.T) {
	jsonOutput = true
	t.Cleanup(func() { jsonOutput = false })

	svcs.Status = &mockStatusReader{items: []map[string]any{
		{"context_name": "acme-prod", "liveness": "live"},
		{"context_name": "beta-dev", "liveness": "dead"},
		{"context_name": "gamma-stg", "liveness": "live"},
	}}
	svcs.Exec = &mockClusterExec{results: map[string]domain.ExecResult{
		"acme-prod": {Context: "acme-prod", OK: true, Stdout: "node-1\n"},
		"gamma-stg": {Context: "gamma-stg", OK: false, Stderr: "denied", ExitCode: 1},
	}}

	var buf bytes.Buffer
	execCmd.SetOut(&buf)
	t.Cleanup(func() { execCmd.SetOut(nil) })

	err := runExec(execCmd, []string{"get", "nodes"})
	require.NoError(t, err)

	var got []map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.Len(t, got, 2)

	byCtx := map[string]map[string]any{}
	for _, r := range got {
		byCtx[r["context"].(string)] = r
	}
	assert.True(t, byCtx["acme-prod"]["ok"].(bool))
	assert.Equal(t, "node-1\n", byCtx["acme-prod"]["stdout"])
	assert.False(t, byCtx["gamma-stg"]["ok"].(bool))
	assert.Equal(t, float64(1), byCtx["gamma-stg"]["exit_code"])
}

func TestRunExec_LiveContexts_Text_PrintsPerContextHeader(t *testing.T) {
	jsonOutput = false
	svcs.Status = &mockStatusReader{items: []map[string]any{
		{"context_name": "acme-prod", "liveness": "live"},
	}}
	svcs.Exec = &mockClusterExec{results: map[string]domain.ExecResult{
		"acme-prod": {Context: "acme-prod", OK: true, Stdout: "v1.28\n"},
	}}

	var buf bytes.Buffer
	execCmd.SetOut(&buf)
	t.Cleanup(func() { execCmd.SetOut(nil) })

	err := runExec(execCmd, []string{"version"})
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "=== acme-prod ===")
	assert.Contains(t, out, "v1.28")
}

func TestRunExec_FiltersOutNonLiveContextsBeforeExec(t *testing.T) {
	jsonOutput = true
	t.Cleanup(func() { jsonOutput = false })

	svcs.Status = &mockStatusReader{items: []map[string]any{
		{"context_name": "live-one", "liveness": "live"},
		{"context_name": "dead-one", "liveness": "dead"},
	}}
	svcs.Exec = &mockClusterExec{}

	var buf bytes.Buffer
	execCmd.SetOut(&buf)
	t.Cleanup(func() { execCmd.SetOut(nil) })

	err := runExec(execCmd, []string{"get", "nodes"})
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "live-one")
	assert.False(t, strings.Contains(out, "dead-one"))
}
