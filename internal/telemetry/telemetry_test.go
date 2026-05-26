package telemetry_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systemframe/k3ctx/internal/telemetry"
)

func TestRecord_WritesJSONLEvent(t *testing.T) {
	dir := t.TempDir()

	w, err := telemetry.NewWriter(dir, 10*1024*1024, 3)
	require.NoError(t, err)
	defer w.Close()

	before := time.Now().UTC().Truncate(time.Second)
	err = w.Record(telemetry.Event{
		Cmd:        "connect",
		Args:       []string{"--context", "sf-prd-us-00001"},
		Flags:      []string{"--context"},
		DurationMs: 1240,
		Ok:         true,
		Version:    "0.1.0",
	})
	require.NoError(t, err)
	after := time.Now().UTC().Add(time.Second)

	data, err := os.ReadFile(filepath.Join(dir, "telemetry.jsonl"))
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	assert.Len(t, lines, 1)

	var evt map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &evt))

	ts, err := time.Parse(time.RFC3339, evt["ts"].(string))
	require.NoError(t, err)
	assert.True(t, !ts.Before(before) && !ts.After(after))

	assert.Equal(t, "connect", evt["cmd"])
	assert.Equal(t, true, evt["ok"])
	assert.Equal(t, float64(1240), evt["duration_ms"])
	assert.Equal(t, "0.1.0", evt["version"])
	assert.Nil(t, evt["error"])

	args := evt["args"].([]any)
	assert.Equal(t, []any{"--context", "sf-prd-us-00001"}, args)
}

func TestRecord_WritesErrorField(t *testing.T) {
	dir := t.TempDir()

	w, err := telemetry.NewWriter(dir, 10*1024*1024, 3)
	require.NoError(t, err)
	defer w.Close()

	err = w.Record(telemetry.Event{
		Cmd:        "connect",
		Args:       []string{"--context", "sf-prd-us-00001"},
		Flags:      []string{"--context"},
		DurationMs: 300,
		Ok:         false,
		Error:      "SSH dial tcp 10.0.0.1:22: connection refused",
		Version:    "0.1.0",
	})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "telemetry.jsonl"))
	require.NoError(t, err)

	var evt map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(string(data))), &evt))

	assert.Equal(t, false, evt["ok"])
	assert.Equal(t, "SSH dial tcp 10.0.0.1:22: connection refused", evt["error"])
}

func TestRecord_RotatesAtSizeLimit(t *testing.T) {
	dir := t.TempDir()

	w, err := telemetry.NewWriter(dir, 100, 3) // 100 bytes max
	require.NoError(t, err)
	defer w.Close()

	for i := 0; i < 10; i++ {
		err = w.Record(telemetry.Event{
			Cmd:        "status",
			DurationMs: 10,
			Ok:         true,
			Version:    "0.1.0",
		})
		require.NoError(t, err)
	}

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}

	// At least one rotation happened
	assert.Greater(t, len(names), 1, "expected rotation, got files: %v", names)
}

func TestRecord_RespectsMaxFiles(t *testing.T) {
	dir := t.TempDir()

	w, err := telemetry.NewWriter(dir, 50, 3) // 50 bytes max, 3 files
	require.NoError(t, err)
	defer w.Close()

	for i := 0; i < 50; i++ {
		_ = w.Record(telemetry.Event{
			Cmd:        "status",
			DurationMs: 10,
			Ok:         true,
			Version:    "0.1.0",
		})
	}

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	assert.LessOrEqual(t, len(entries), 3, "expected at most 3 files, got: %d", len(entries))
}
