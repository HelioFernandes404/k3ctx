package usecases

import (
	"github.com/systemframe/k3ctx/internal/domain"
)

// SetCurrentContext switches the active kubectl context.
// When requireConfirmation is true and confirmed is false, returns a confirmation_required error.
func SetCurrentContext(contextName string, switcher domain.ContextSwitcher, requireConfirmation, confirmed bool) *domain.OperationError {
	if requireConfirmation && !confirmed {
		err := domain.OperationError{
			Code:    "confirmation_required",
			Message: "User confirmation required to switch context",
		}
		return &err
	}
	return switcher.SwitchContext(contextName)
}
