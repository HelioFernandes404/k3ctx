package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/systemframe/k3ctx/internal/domain"
)

func TestNetBirdErrorCodes_ExistAndSerialise(t *testing.T) {
	cases := []struct {
		code    string
		wantKey string
	}{
		{domain.ErrCodeNetBirdNotReady, "NETBIRD_NOT_READY"},
		{domain.ErrCodePeerNotConnected, "PEER_NOT_CONNECTED"},
		{domain.ErrCodePeerNotFound, "PEER_NOT_FOUND"},
	}
	for _, tc := range cases {
		opErr := domain.OperationError{Code: tc.code, Message: "msg", Hint: "hint"}
		pub := opErr.ToPublicDict()
		assert.Equal(t, tc.wantKey, pub["code"])
		assert.Equal(t, tc.wantKey, opErr.Code)
	}
}
