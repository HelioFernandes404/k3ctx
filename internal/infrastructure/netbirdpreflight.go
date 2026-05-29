package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/systemframe/k3ctx/internal/domain"
	"github.com/systemframe/k3ctx/internal/netbird"
)

const preflightTimeout = 3 * time.Second

// NetBirdPreflightChecker validates NetBird daemon and target peer readiness.
// CheckDaemonReady must be called before CheckPeerReady; the parsed peer list
// is cached between the two calls so `netbird status --json` runs only once.
type NetBirdPreflightChecker struct {
	binPath string
	skipped bool
	peers   []netbird.Peer
}

// NewNetBirdPreflightChecker creates a checker using the given binary path.
// An empty binPath defaults to "netbird" resolved from $PATH.
func NewNetBirdPreflightChecker(binPath string) *NetBirdPreflightChecker {
	if binPath == "" {
		binPath = "netbird"
	}
	return &NetBirdPreflightChecker{binPath: binPath}
}

// CheckDaemonReady runs `netbird status --json` and validates daemon state.
// Returns nil and sets the internal skip flag when the binary is absent or
// skipCheck is true.
func (c *NetBirdPreflightChecker) CheckDaemonReady(skipCheck bool) error {
	if skipCheck {
		c.skipped = true
		return nil
	}

	resolved, lookErr := exec.LookPath(c.binPath)
	if lookErr != nil {
		c.skipped = true
		return nil
	}

	data, runErr := runNetBirdStatusCmd(resolved)
	if runErr != nil {
		if errors.Is(runErr, context.DeadlineExceeded) {
			return &domain.OperationError{
				Code:    domain.ErrCodeNetBirdNotReady,
				Message: "NetBird status check timed out",
				Hint:    "Run: netbird status",
			}
		}
		return &domain.OperationError{
			Code:    domain.ErrCodeNetBirdNotReady,
			Message: "NetBird daemon is not running",
			Hint:    "Run: netbird service start && netbird up",
		}
	}

	result, parseErr := netbird.ParseStatus(data)
	if parseErr != nil {
		return &domain.OperationError{
			Code:    domain.ErrCodeNetBirdNotReady,
			Message: "NetBird status check failed",
			Hint:    "Run: netbird status",
		}
	}

	c.peers = result.Peers

	if result.DaemonStatus != "Connected" {
		status := result.DaemonStatus
		if status == "" {
			status = "Unknown"
		}
		return &domain.OperationError{
			Code:    domain.ErrCodeNetBirdNotReady,
			Message: "NetBird is not authenticated",
			Hint:    fmt.Sprintf("NetBird daemon status: %s. Run: netbird up", status),
		}
	}

	return nil
}

// CheckPeerReady looks up fqdn in the peer list cached by CheckDaemonReady.
// Returns nil when skipped, fqdn is empty, or the peer is Connected.
func (c *NetBirdPreflightChecker) CheckPeerReady(fqdn string, skipCheck bool) error {
	if skipCheck || c.skipped || fqdn == "" {
		return nil
	}

	for _, p := range c.peers {
		if p.FQDN == fqdn {
			if p.Status != "Connected" {
				return &domain.OperationError{
					Code:    domain.ErrCodePeerNotConnected,
					Message: "NetBird target peer is not connected",
					Hint:    fmt.Sprintf("NetBird peer %s is %s. Check peer connectivity in NetBird dashboard", fqdn, p.Status),
				}
			}
			return nil
		}
	}

	return &domain.OperationError{
		Code:    domain.ErrCodePeerNotFound,
		Message: "NetBird target peer not found",
		Hint:    fmt.Sprintf("NetBird peer %s not found. Verify the host is enrolled in your NetBird network", fqdn),
	}
}

func runNetBirdStatusCmd(binPath string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), preflightTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, binPath, "status", "--json").Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, context.DeadlineExceeded
		}
		return nil, err
	}
	return out, nil
}
