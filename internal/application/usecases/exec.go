package usecases

import (
	"context"
	"sync"
	"time"

	"github.com/systemframe/k3ctx/internal/application"
)

// ExecOnContexts runs args against each contextName in parallel with the given timeout per context.
func ExecOnContexts(contextNames []string, args []string, timeout time.Duration, executer application.ClusterExec) []application.ExecResult {
	if len(contextNames) == 0 {
		return []application.ExecResult{}
	}

	results := make([]application.ExecResult, len(contextNames))
	var wg sync.WaitGroup
	for i, name := range contextNames {
		wg.Add(1)
		go func(idx int, contextName string) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			results[idx] = executer.ExecOnContext(ctx, contextName, args)
		}(i, name)
	}
	wg.Wait()
	return results
}
