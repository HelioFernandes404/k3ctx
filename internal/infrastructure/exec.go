package infrastructure

import (
	"bytes"
	"context"
	"os/exec"

	"github.com/systemframe/k3ctx/internal/domain"
)

// LocalClusterExec runs kubectl commands against named contexts via os/exec.
type LocalClusterExec struct{}

// ExecOnContext runs `kubectl --context <contextName> <args...>` and returns its captured result.
func (LocalClusterExec) ExecOnContext(ctx context.Context, contextName string, args []string) domain.ExecResult {
	cmdArgs := append([]string{"--context", contextName}, args...)
	cmd := exec.CommandContext(ctx, "kubectl", cmdArgs...) //nolint:gosec // kubectl with caller-provided args is the whole point of exec

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	return domain.ExecResult{
		Context:  contextName,
		OK:       err == nil,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}
