package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

// SessionOwnership coordinates character ownership across gateway replicas.
// Account admission is owned by the character service.
type SessionOwnership interface {
	Register(token string, evict func(context.Context)) func()
	ClaimCharacter(context.Context, uint64, string) error
	ReleaseCharacter(context.Context, uint64, string) error
}

func newSessionToken() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value[:])
}
