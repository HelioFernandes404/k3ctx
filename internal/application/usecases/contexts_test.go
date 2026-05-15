package usecases_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/systemframe/k3ctx/internal/application/usecases"
	"github.com/systemframe/k3ctx/internal/domain"
)

type stubSwitcher struct {
	result *domain.OperationError
	calls  []string
}

func (s *stubSwitcher) SwitchContext(name string) *domain.OperationError {
	s.calls = append(s.calls, name)
	return s.result
}

func TestSetCurrentContext_RequiresExplicitConfirmation(t *testing.T) {
	sw := &stubSwitcher{}
	err := usecases.SetCurrentContext("acme-prod", sw, true, false)
	assert.NotNil(t, err)
	assert.Equal(t, "confirmation_required", err.Code)
	assert.Empty(t, sw.calls)
}

func TestSetCurrentContext_DelegatesToSwitcherWhenConfirmed(t *testing.T) {
	sw := &stubSwitcher{}
	err := usecases.SetCurrentContext("acme-prod", sw, true, true)
	assert.Nil(t, err)
	assert.Equal(t, []string{"acme-prod"}, sw.calls)
}

func TestSetCurrentContext_DelegatesWhenConfirmationNotRequired(t *testing.T) {
	sw := &stubSwitcher{}
	err := usecases.SetCurrentContext("acme-prod", sw, false, false)
	assert.Nil(t, err)
	assert.Equal(t, []string{"acme-prod"}, sw.calls)
}

func TestSetCurrentContext_PropagatesSwitcherError(t *testing.T) {
	opErr := &domain.OperationError{Code: "kubectl_context_failed", Message: "Failed to switch kubectl context"}
	sw := &stubSwitcher{result: opErr}
	err := usecases.SetCurrentContext("acme-prod", sw, false, false)
	assert.Equal(t, opErr, err)
	assert.Equal(t, []string{"acme-prod"}, sw.calls)
}

func TestSetCurrentContext_DoesNotCallSwitcherWithoutConfirmation(t *testing.T) {
	sw := &stubSwitcher{}
	usecases.SetCurrentContext("acme-prod", sw, true, false)
	assert.Empty(t, sw.calls)
}
