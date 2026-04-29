package repo

import (
	"context"
	"time"
)

// Channel represents a WoW channel
type Channel struct {
	ChannelID  uint32
	Name       string
	Team       uint32 // 0=Alliance, 1=Horde, 2=Neutral
	Announce   bool
	Ownership  bool
	Password   string
	LastUsed   time.Time
}

// ChannelRights represents channel moderation settings
type ChannelRights struct {
	Name         string
	Flags        uint32
	SpeakDelay   uint32
	JoinMessage  string
	DelayMessage string
	Moderators   []uint32
}

// ChannelBan represents a channel ban entry
type ChannelBan struct {
	ChannelID  uint32
	PlayerGUID uint64
	BanTime    time.Time
}

// ChannelMember represents a player's membership in a channel (in-memory only)
type ChannelMember struct {
	PlayerGUID uint64
	PlayerName string
	Flags      uint8 // MEMBER_FLAG_OWNER, MEMBER_FLAG_MODERATOR, etc.
}

// ChannelsRepo provides database access for channel data
type ChannelsRepo interface {
	// Channel CRUD
	CreateChannel(ctx context.Context, realmID uint32, channel *Channel) error
	GetChannelByID(ctx context.Context, realmID uint32, channelID uint32) (*Channel, error)
	GetChannelByName(ctx context.Context, realmID uint32, name string, team uint32) (*Channel, error)
	GetAllChannels(ctx context.Context, realmID uint32) ([]Channel, error)
	UpdateChannel(ctx context.Context, realmID uint32, channel *Channel) error
	DeleteChannel(ctx context.Context, realmID uint32, channelID uint32) error
	UpdateChannelLastUsed(ctx context.Context, realmID uint32, channelID uint32) error
	CleanOldChannels(ctx context.Context, realmID uint32, olderThan time.Time) error

	// Channel rights
	GetChannelRights(ctx context.Context, realmID uint32, name string) (*ChannelRights, error)
	SetChannelRights(ctx context.Context, realmID uint32, rights *ChannelRights) error

	// Channel bans
	AddChannelBan(ctx context.Context, realmID uint32, ban *ChannelBan) error
	RemoveChannelBan(ctx context.Context, realmID uint32, channelID uint32, playerGUID uint64) error
	GetChannelBans(ctx context.Context, realmID uint32, channelID uint32) ([]ChannelBan, error)
	IsPlayerBanned(ctx context.Context, realmID uint32, channelID uint32, playerGUID uint64) (bool, error)
	CleanExpiredBans(ctx context.Context, realmID uint32) error

	// Channel members
	SaveChannelMember(ctx context.Context, realmID uint32, channelID uint32, member *ChannelMember) error
	RemoveChannelMember(ctx context.Context, realmID uint32, channelID uint32, playerGUID uint64) error
	LoadChannelMembers(ctx context.Context, realmID uint32, channelID uint32) ([]ChannelMember, error)
	UpdateMemberFlags(ctx context.Context, realmID uint32, channelID uint32, playerGUID uint64, flags uint8) error
}
