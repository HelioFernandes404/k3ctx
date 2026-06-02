package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteError_JSONMode(t *testing.T) {
	jsonOutput = true
	t.Cleanup(func() { jsonOutput = false })

	var buf bytes.Buffer
	writeError(&buf, "TEST_CODE", "something went wrong", "try this instead")

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))

	errMap, ok := got["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "TEST_CODE", errMap["code"])
	assert.Equal(t, "something went wrong", errMap["message"])
	assert.Equal(t, "try this instead", errMap["hint"])
}

func TestWriteError_JSONMode_NoOkField(t *testing.T) {
	jsonOutput = true
	t.Cleanup(func() { jsonOutput = false })

	var buf bytes.Buffer
	writeError(&buf, "X", "msg", "")

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	_, hasOk := got["ok"]
	assert.False(t, hasOk, "pure JSON errors must not include ok field")
}

func TestWriteError_TextMode_WithHint(t *testing.T) {
	jsonOutput = false

	var buf bytes.Buffer
	writeError(&buf, "X", "the message", "the hint")

	out := buf.String()
	assert.Contains(t, out, "the message")
	assert.Contains(t, out, "the hint")
}

func TestWriteError_TextMode_NoHint(t *testing.T) {
	jsonOutput = false

	var buf bytes.Buffer
	writeError(&buf, "X", "the message", "")

	out := buf.String()
	assert.Contains(t, out, "the message")
	assert.NotContains(t, out, "\n\n")
}

func TestNewExitErr_Fields(t *testing.T) {
	e := newExitErr(4, "NO_MATCH", "nothing found", "try a broader query")

	assert.Equal(t, 4, e.Code)
	assert.Equal(t, "NO_MATCH", e.ECode)
	assert.Equal(t, "nothing found", e.Msg)
	assert.Equal(t, "try a broader query", e.Hint)
	assert.Equal(t, "nothing found", e.Error())
}

func TestExitErr_ImplementsError(t *testing.T) {
	var err error = newExitErr(1, "ERR", "msg", "")
	assert.EqualError(t, err, "msg")
}
