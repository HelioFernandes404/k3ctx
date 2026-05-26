package infrastructure_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/systemframe/k3ctx/internal/infrastructure"
)

func TestNetBirdRefresher_success(t *testing.T) {
	r := infrastructure.NetBirdInventoryRefresher{
		StatusFn: func() error { return nil },
	}
	ok, msg := r.Refresh("")
	assert.True(t, ok)
	assert.Contains(t, msg, "refreshed")
}

func TestNetBirdRefresher_failure(t *testing.T) {
	r := infrastructure.NetBirdInventoryRefresher{
		StatusFn: func() error { return assert.AnError },
	}
	ok, msg := r.Refresh("")
	assert.False(t, ok)
	assert.Contains(t, msg, "assert.AnError")
}
