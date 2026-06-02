package infrastructure

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/systemframe/k3ctx/internal/domain"
)

// KubectlContextSwitcher switches kubectl contexts via the kubectl CLI.
type KubectlContextSwitcher struct{}

// SwitchContext invokes `kubectl config use-context` for the given context name.
func (KubectlContextSwitcher) SwitchContext(contextName string) *domain.OperationError {
	out, err := exec.CommandContext(context.Background(), "kubectl", "config", "use-context", contextName).CombinedOutput() //nolint:gosec // kubectl with fixed subcommand and context name
	if err != nil {
		msg := fmt.Sprintf("Failed to switch kubectl context to %s", contextName)
		_ = strings.TrimSpace(string(out)) // do not expose in public error
		return &domain.OperationError{
			Code:    "kubectl_context_failed",
			Message: msg,
		}
	}
	return nil
}
