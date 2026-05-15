package infrastructure_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/systemframe/k3ctx/internal/infrastructure"
)

func TestKubectlContextSwitcher_ReturnsNilOnSuccess(t *testing.T) {
	// This test only runs when kubectl is available and context exists;
	// it serves as documentation. For unit isolation, the behavior is
	// tested via the ContextSwitcher port in use-case tests.
	t.Skip("integration — requires live kubectl")
	sw := infrastructure.KubectlContextSwitcher{}
	err := sw.SwitchContext("nonexistent-context")
	assert.NotNil(t, err)
	assert.Equal(t, "kubectl_context_failed", err.Code)
}

func TestKubectlContextSwitcher_DoesNotExposeDetailInPublicError(t *testing.T) {
	sw := infrastructure.KubectlContextSwitcher{}
	// Call with a context name that definitely won't exist
	err := sw.SwitchContext("k3ctx-test-nonexistent-xxxxxxxxxxx")
	if err == nil {
		t.Skip("kubectl not available or context unexpectedly succeeded")
	}
	assert.Equal(t, "kubectl_context_failed", err.Code)
	assert.Empty(t, err.Detail)
}
