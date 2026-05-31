package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunSchema_TopLevelKeys(t *testing.T) {
	var buf bytes.Buffer
	schemaCmd.SetOut(&buf)
	t.Cleanup(func() { schemaCmd.SetOut(nil) })

	err := runSchema(schemaCmd, nil)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	assert.Equal(t, true, got["ok"])
	assert.Equal(t, "schema", got["command"])

	data := got["data"].(map[string]any)
	assert.NotEmpty(t, data["version"])
	assert.NotEmpty(t, data["behavior"])
	assert.NotEmpty(t, data["exit_codes"])
	assert.NotEmpty(t, data["error_codes"])
	assert.NotEmpty(t, data["commands"])
}

func TestRunSchema_ContainsConnectWithDryRunFlag(t *testing.T) {
	var buf bytes.Buffer
	schemaCmd.SetOut(&buf)
	t.Cleanup(func() { schemaCmd.SetOut(nil) })

	require.NoError(t, runSchema(schemaCmd, nil))

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	data := got["data"].(map[string]any)
	commands := data["commands"].([]any)

	var connectEntry map[string]any
	for _, c := range commands {
		entry := c.(map[string]any)
		if entry["name"] == "connect" {
			connectEntry = entry
			break
		}
	}
	require.NotNil(t, connectEntry, "connect command should be present")

	flags := connectEntry["flags"].([]any)
	flagNames := make([]string, 0, len(flags))
	for _, f := range flags {
		fm := f.(map[string]any)
		flagNames = append(flagNames, fm["name"].(string))
	}
	assert.Contains(t, flagNames, "dry-run")
	assert.Contains(t, flagNames, "all-hosts")
}

func TestRunSchema_ErrorCodesIncludeKnownCodes(t *testing.T) {
	var buf bytes.Buffer
	schemaCmd.SetOut(&buf)
	t.Cleanup(func() { schemaCmd.SetOut(nil) })

	require.NoError(t, runSchema(schemaCmd, nil))

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	data := got["data"].(map[string]any)
	codes := data["error_codes"].([]any)

	codeStrings := make([]string, 0, len(codes))
	for _, c := range codes {
		codeStrings = append(codeStrings, c.(string))
	}
	assert.Contains(t, codeStrings, "NO_MATCH")
	assert.Contains(t, codeStrings, "AMBIGUOUS_MATCH")
	assert.Contains(t, codeStrings, "CONNECTION_FAILED")
}
