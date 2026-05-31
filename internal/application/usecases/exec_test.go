package usecases_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systemframe/k3ctx/internal/application"
	"github.com/systemframe/k3ctx/internal/application/usecases"
)

type stubClusterExec struct {
	mu      sync.Mutex
	called  []string
	results map[string]application.ExecResult
}

func (s *stubClusterExec) ExecOnContext(_ context.Context, contextName string, _ []string) application.ExecResult {
	s.mu.Lock()
	s.called = append(s.called, contextName)
	s.mu.Unlock()
	if r, ok := s.results[contextName]; ok {
		return r
	}
	return application.ExecResult{Context: contextName, OK: true}
}

func TestExecOnContexts_EmptyReturnsEmptySlice(t *testing.T) {
	stub := &stubClusterExec{}
	results := usecases.ExecOnContexts(nil, []string{"get", "nodes"}, time.Second, stub)
	assert.Empty(t, results)
	assert.Empty(t, stub.called)
}

func TestExecOnContexts_CallsExecForEachContext(t *testing.T) {
	stub := &stubClusterExec{}
	results := usecases.ExecOnContexts([]string{"ctx-a", "ctx-b"}, []string{"version"}, time.Second, stub)
	require.Len(t, results, 2)
	assert.ElementsMatch(t, []string{"ctx-a", "ctx-b"}, stub.called)
}

func TestExecOnContexts_ReturnsAllResultsOnPartialFailure(t *testing.T) {
	stub := &stubClusterExec{
		results: map[string]application.ExecResult{
			"ctx-ok":  {Context: "ctx-ok", OK: true, Stdout: "v1.28"},
			"ctx-bad": {Context: "ctx-bad", OK: false, ExitCode: 1},
		},
	}
	results := usecases.ExecOnContexts([]string{"ctx-ok", "ctx-bad"}, []string{"version"}, time.Second, stub)
	require.Len(t, results, 2)

	byCtx := map[string]application.ExecResult{}
	for _, r := range results {
		byCtx[r.Context] = r
	}
	assert.True(t, byCtx["ctx-ok"].OK)
	assert.False(t, byCtx["ctx-bad"].OK)
}
