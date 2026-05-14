package infrastructure

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/systemframe/k3ctx/internal/domain"
)

// KubectlContextSwitcher switches kubectl contexts via the kubectl CLI.
type KubectlContextSwitcher struct{}

func (KubectlContextSwitcher) SwitchContext(contextName string) *domain.OperationError {
	out, err := exec.Command("kubectl", "config", "use-context", contextName).CombinedOutput()
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
