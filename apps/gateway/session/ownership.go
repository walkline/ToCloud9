package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

// SessionOwnership coordinates account and character ownership across all
// gateway replicas. Implementations must fence releases by token and durably
// notify a previous owner when a claim replaces it.
type SessionOwnership interface {
	Register(token string, evict func(context.Context)) func()
	ClaimAccount(context.Context, uint32, string) error
	ClaimCharacter(context.Context, uint64, string) error
	ReleaseAccount(context.Context, uint32, string) error
	ReleaseCharacter(context.Context, uint64, string) error
}

func newSessionToken() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value[:])
}
