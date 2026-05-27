package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systemframe/k3ctx/internal/telemetry"
)

func TestRecordErrorTelemetry_CapturesCmdArgsAndFlags(t *testing.T) {
	dir := t.TempDir()
	w, err := telemetry.NewWriter(dir, 10*1024*1024, 3)
	require.NoError(t, err)

	cmd := &cobra.Command{Use: "connect"}
	cmd.Flags().String("context", "", "context name")
	require.NoError(t, cmd.Flags().Set("context", "sf-tst-sp-00003"))

	telWriter = w
	calledCobraCmd = cmd
	calledArgs = []string{}
	t.Cleanup(func() {
		telWriter = nil
		calledCobraCmd = nil
		calledArgs = nil
	})

	recordErrorTelemetry(errors.New("Cluster connection failed"))

	data, err := os.ReadFile(filepath.Join(dir, "telemetry.jsonl"))
	require.NoError(t, err)

	var evt map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(string(data))), &evt))

	assert.Equal(t, "connect", evt["cmd"], "cmd must be captured on error path")
	assert.Equal(t, false, evt["ok"])
	assert.Equal(t, "Cluster connection failed", evt["error"])

	require.NotNil(t, evt["flags"], "flags must not be null on error path")
	flags := evt["flags"].([]any)
	assert.Contains(t, flags, "--context", "used flags must be captured on error path")
}
