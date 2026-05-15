package usecases_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systemframe/k3ctx/internal/application/usecases"
)

type stubStatusReader struct {
	statusItems []map[string]any
	netResult   map[string]any
	listCalls   int
	validateCalls []string
}

func (s *stubStatusReader) ListContextStatus() ([]map[string]any, error) {
	s.listCalls++
	return s.statusItems, nil
}

func (s *stubStatusReader) ValidateContextNetwork(name string) (map[string]any, error) {
	s.validateCalls = append(s.validateCalls, name)
	return s.netResult, nil
}

func TestListContextStatus_DelegatesToReader(t *testing.T) {
	expected := []map[string]any{{"name": "acme-prod", "is_current": true}}
	reader := &stubStatusReader{statusItems: expected}

	result, err := usecases.ListContextStatus(reader)

	require.NoError(t, err)
	assert.Equal(t, expected, result)
	assert.Equal(t, 1, reader.listCalls)
}

func TestListContextStatus_ReturnsEmptyWhenNoContexts(t *testing.T) {
	reader := &stubStatusReader{}
	result, err := usecases.ListContextStatus(reader)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestListContextStatus_ReturnsMultipleContexts(t *testing.T) {
	items := []map[string]any{
		{"name": "acme-prod", "is_current": true, "tunnel_running": true},
		{"name": "acme-dev", "is_current": false, "tunnel_running": false},
	}
	reader := &stubStatusReader{statusItems: items}
	result, err := usecases.ListContextStatus(reader)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "acme-prod", result[0]["name"])
	assert.Equal(t, "acme-dev", result[1]["name"])
}

func TestValidateContextNetwork_DelegatesToReader(t *testing.T) {
	expected := map[string]any{"context_name": "acme-prod", "ok": true, "warning": nil}
	reader := &stubStatusReader{netResult: expected}

	result, err := usecases.ValidateContextNetwork("acme-prod", reader)

	require.NoError(t, err)
	assert.Equal(t, expected, result)
	assert.Equal(t, []string{"acme-prod"}, reader.validateCalls)
}

func TestValidateContextNetwork_PassesContextNameToReader(t *testing.T) {
	reader := &stubStatusReader{netResult: map[string]any{"ok": true}}
	_, _ = usecases.ValidateContextNetwork("beta-staging", reader)
	assert.Equal(t, []string{"beta-staging"}, reader.validateCalls)
}

func TestValidateContextNetwork_ReturnsReaderFailure(t *testing.T) {
	expected := map[string]any{"ok": false, "warning": "VPN required"}
	reader := &stubStatusReader{netResult: expected}
	result, err := usecases.ValidateContextNetwork("acme-dev", reader)
	require.NoError(t, err)
	assert.Equal(t, false, result["ok"])
	assert.Equal(t, "VPN required", result["warning"])
}
