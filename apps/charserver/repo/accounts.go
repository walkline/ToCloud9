package repo

import "context"

const (
	RealIDFriendStatusPending  uint8 = 1
	RealIDFriendStatusAccepted uint8 = 2
)

type Account struct {
	ID       uint32
	Username string
	Email    string
}

type RealIDFriendRelation struct {
	AccountID          uint32
	FriendAccountID    uint32
	RequesterAccountID uint32
	Status             uint8
	Note               string
}

type Accounts interface {
	AccountByEmail(ctx context.Context, email string) (*Account, error)
	RequestRealIDFriend(ctx context.Context, requesterAccountID uint32, addresseeAccountID uint32, note string) (*RealIDFriendRelation, error)
	AcceptRealIDFriend(ctx context.Context, accountID uint32, requesterAccountID uint32, note string) (*RealIDFriendRelation, error)
	RemoveRealIDFriend(ctx context.Context, accountID uint32, friendAccountID uint32) error
	UpdateRealIDFriendNote(ctx context.Context, accountID uint32, friendAccountID uint32, note string) error
	AcceptedRealIDFriends(ctx context.Context, accountID uint32) ([]*RealIDFriendRelation, error)
}
