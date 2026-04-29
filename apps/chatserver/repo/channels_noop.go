package repo

import (
	"context"
	"sync/atomic"
	"time"
)

// channelsNoopRepo is a no-op implementation of ChannelsRepo
// Used when database persistence is not needed (channels are ephemeral)
type channelsNoopRepo struct {
	nextChannelID uint32
}

func NewChannelsNoopRepo() ChannelsRepo {
	return &channelsNoopRepo{
		nextChannelID: 1,
	}
}

func (r *channelsNoopRepo) CreateChannel(ctx context.Context, realmID uint32, channel *Channel) error {
	// If channelID is provided (from worldserver), use it; otherwise generate new ID
	if channel.ChannelID == 0 {
		channel.ChannelID = atomic.AddUint32(&r.nextChannelID, 1)
	}
	// else: reuse the provided channelID (from worldserver for territorial channels)
	return nil
}

func (r *channelsNoopRepo) GetChannelByID(ctx context.Context, realmID uint32, channelID uint32) (*Channel, error) {
	return nil, nil
}

func (r *channelsNoopRepo) GetChannelByName(ctx context.Context, realmID uint32, name string, team uint32) (*Channel, error) {
	return nil, nil
}

func (r *channelsNoopRepo) GetAllChannels(ctx context.Context, realmID uint32) ([]Channel, error) {
	return nil, nil
}

func (r *channelsNoopRepo) UpdateChannel(ctx context.Context, realmID uint32, channel *Channel) error {
	return nil
}

func (r *channelsNoopRepo) DeleteChannel(ctx context.Context, realmID uint32, channelID uint32) error {
	return nil
}

func (r *channelsNoopRepo) UpdateChannelLastUsed(ctx context.Context, realmID uint32, channelID uint32) error {
	return nil
}

func (r *channelsNoopRepo) CleanOldChannels(ctx context.Context, realmID uint32, olderThan time.Time) error {
	return nil
}

func (r *channelsNoopRepo) GetChannelRights(ctx context.Context, realmID uint32, name string) (*ChannelRights, error) {
	return nil, nil
}

func (r *channelsNoopRepo) SetChannelRights(ctx context.Context, realmID uint32, rights *ChannelRights) error {
	return nil
}

func (r *channelsNoopRepo) AddChannelBan(ctx context.Context, realmID uint32, ban *ChannelBan) error {
	return nil
}

func (r *channelsNoopRepo) RemoveChannelBan(ctx context.Context, realmID uint32, channelID uint32, playerGUID uint64) error {
	return nil
}

func (r *channelsNoopRepo) GetChannelBans(ctx context.Context, realmID uint32, channelID uint32) ([]ChannelBan, error) {
	return nil, nil
}

func (r *channelsNoopRepo) IsPlayerBanned(ctx context.Context, realmID uint32, channelID uint32, playerGUID uint64) (bool, error) {
	return false, nil
}

func (r *channelsNoopRepo) CleanExpiredBans(ctx context.Context, realmID uint32) error {
	return nil
}

func (r *channelsNoopRepo) SaveChannelMember(ctx context.Context, realmID uint32, channelID uint32, member *ChannelMember) error {
	return nil
}

func (r *channelsNoopRepo) RemoveChannelMember(ctx context.Context, realmID uint32, channelID uint32, playerGUID uint64) error {
	return nil
}

func (r *channelsNoopRepo) LoadChannelMembers(ctx context.Context, realmID uint32, channelID uint32) ([]ChannelMember, error) {
	return nil, nil
}

func (r *channelsNoopRepo) UpdateMemberFlags(ctx context.Context, realmID uint32, channelID uint32, playerGUID uint64, flags uint8) error {
	return nil
}
