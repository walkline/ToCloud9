package repo

import "context"

type Account struct {
	ID             uint32
	Username       string
	Salt           []byte
	Verifier       []byte
	SessionKeyAuth []byte
	Locked         bool
	LastIP         string
}

type AccountRepo interface {
	AccountByUserName(ctx context.Context, username string) (*Account, error)
}
