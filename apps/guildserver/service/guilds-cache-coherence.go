package service

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/guildserver/repo"
	"github.com/walkline/ToCloud9/shared/events"
)

// guildsCacheWithCoherence decorates GuildsCache to publish peer invalidations
// after successful local mutations. UI GuildServiceProducer events stay separate.
//
// Decoupling: domain services only see GuildsCache; NATS peer bus is an adapter
// concern wired at process bootstrap (main).
type guildsCacheWithCoherence struct {
	GuildsCache
	producer  events.GuildCacheProducer
	serviceID string
}

// NewGuildsCacheWithCoherence wraps cache so every successful roster mutation
// notifies peer guildserver replicas via guild.cache.invalidate.
func NewGuildsCacheWithCoherence(inner GuildsCache, producer events.GuildCacheProducer, serviceID string) GuildsCache {
	if producer == nil {
		producer = events.NopGuildCacheProducer{}
	}
	if peer, ok := inner.(GuildCachePeerSync); ok {
		peer.SetLocalServiceID(serviceID)
	}
	return &guildsCacheWithCoherence{
		GuildsCache: inner,
		producer:    producer,
		serviceID:   serviceID,
	}
}

// resolveMemberGuildID captures guild id before a member-keyed mutation.
// Prefers MySQL source so multi-replica stale index cannot target the wrong guild.
func (c *guildsCacheWithCoherence) resolveMemberGuildID(ctx context.Context, realmID uint32, memberGUID uint64) uint64 {
	if src, ok := c.GuildsCache.(GuildMembershipSource); ok {
		guildID, err := src.GuildIDByRealmAndMemberGUIDFromSource(ctx, realmID, memberGUID)
		if err != nil {
			log.Warn().Err(err).Uint32("realmID", realmID).Uint64("memberGUID", memberGUID).
				Msg("guild cache coherence: resolve member guild from source failed")
		}
		if guildID != 0 {
			return guildID
		}
	}
	guildID, _ := c.GuildsCache.GuildIDByRealmAndMemberGUID(ctx, realmID, memberGUID)
	if guildID == 0 {
		log.Warn().Uint32("realmID", realmID).Uint64("memberGUID", memberGUID).
			Msg("guild cache coherence: member guild id unknown; peer invalidate may be skipped")
	}
	return guildID
}

func (c *guildsCacheWithCoherence) publishInvalidate(realmID uint32, guildID uint64) {
	if guildID == 0 {
		return
	}
	if err := c.producer.InvalidateGuild(&events.GuildCacheInvalidatePayload{
		ServiceID: c.serviceID,
		RealmID:   realmID,
		GuildID:   guildID,
	}); err != nil {
		// Local mutation already committed; peer lag is healed by next ForceRefresh
		// or by the throttled rehydrate path. Do not fail the caller.
		log.Warn().Err(err).Uint32("realmID", realmID).Uint64("guildID", guildID).
			Msg("guild cache coherence: publish invalidate failed")
	}
}

func (c *guildsCacheWithCoherence) CreateGuild(ctx context.Context, realmID uint32, name string, leaderGUID uint64, ranks []repo.GuildRank, memberGUIDs []uint64) (uint64, error) {
	id, err := c.GuildsCache.CreateGuild(ctx, realmID, name, leaderGUID, ranks, memberGUIDs)
	// Publish whenever MySQL created an id, even if local hydrate failed afterward.
	if id != 0 {
		c.publishInvalidate(realmID, id)
	}
	return id, err
}

func (c *guildsCacheWithCoherence) AddGuildMember(ctx context.Context, realmID uint32, member repo.GuildMember) error {
	if err := c.GuildsCache.AddGuildMember(ctx, realmID, member); err != nil {
		return err
	}
	c.publishInvalidate(realmID, member.GuildID)
	return nil
}

func (c *guildsCacheWithCoherence) AcceptGuildInvite(ctx context.Context, realmID uint32, member repo.GuildMember) (uint64, error) {
	guildID, err := c.GuildsCache.AcceptGuildInvite(ctx, realmID, member)
	if err != nil {
		return 0, err
	}
	c.publishInvalidate(realmID, guildID)
	return guildID, nil
}

func (c *guildsCacheWithCoherence) RemoveGuildMember(ctx context.Context, realmID uint32, characterGUID uint64) error {
	guildID := c.resolveMemberGuildID(ctx, realmID, characterGUID)
	if err := c.GuildsCache.RemoveGuildMember(ctx, realmID, characterGUID); err != nil {
		return err
	}
	c.publishInvalidate(realmID, guildID)
	return nil
}

func (c *guildsCacheWithCoherence) SetMessageOfTheDay(ctx context.Context, realmID uint32, guildID uint64, message string) error {
	if err := c.GuildsCache.SetMessageOfTheDay(ctx, realmID, guildID, message); err != nil {
		return err
	}
	c.publishInvalidate(realmID, guildID)
	return nil
}

func (c *guildsCacheWithCoherence) SetGuildEmblem(ctx context.Context, realmID uint32, guildID uint64, emblem repo.GuildEmblem) error {
	if err := c.GuildsCache.SetGuildEmblem(ctx, realmID, guildID, emblem); err != nil {
		return err
	}
	c.publishInvalidate(realmID, guildID)
	return nil
}

func (c *guildsCacheWithCoherence) SetMemberPublicNote(ctx context.Context, realmID uint32, memberGUID uint64, note string) error {
	guildID := c.resolveMemberGuildID(ctx, realmID, memberGUID)
	if err := c.GuildsCache.SetMemberPublicNote(ctx, realmID, memberGUID, note); err != nil {
		return err
	}
	c.publishInvalidate(realmID, guildID)
	return nil
}

func (c *guildsCacheWithCoherence) SetMemberOfficerNote(ctx context.Context, realmID uint32, memberGUID uint64, note string) error {
	guildID := c.resolveMemberGuildID(ctx, realmID, memberGUID)
	if err := c.GuildsCache.SetMemberOfficerNote(ctx, realmID, memberGUID, note); err != nil {
		return err
	}
	c.publishInvalidate(realmID, guildID)
	return nil
}

func (c *guildsCacheWithCoherence) SetMemberRank(ctx context.Context, realmID uint32, memberGUID uint64, rank uint8) error {
	guildID := c.resolveMemberGuildID(ctx, realmID, memberGUID)
	if err := c.GuildsCache.SetMemberRank(ctx, realmID, memberGUID, rank); err != nil {
		return err
	}
	c.publishInvalidate(realmID, guildID)
	return nil
}

func (c *guildsCacheWithCoherence) SetGuildInfo(ctx context.Context, realmID uint32, guildID uint64, info string) error {
	if err := c.GuildsCache.SetGuildInfo(ctx, realmID, guildID, info); err != nil {
		return err
	}
	c.publishInvalidate(realmID, guildID)
	return nil
}

func (c *guildsCacheWithCoherence) UpdateGuildRank(
	ctx context.Context, realmID uint32, guildID uint64,
	rank uint8, name string, rights, moneyPerDay uint32,
) error {
	if err := c.GuildsCache.UpdateGuildRank(ctx, realmID, guildID, rank, name, rights, moneyPerDay); err != nil {
		return err
	}
	c.publishInvalidate(realmID, guildID)
	return nil
}

func (c *guildsCacheWithCoherence) AddGuildRank(ctx context.Context, realmID uint32, guildID uint64, rank uint8, name string, rights, moneyPerDay uint32) error {
	if err := c.GuildsCache.AddGuildRank(ctx, realmID, guildID, rank, name, rights, moneyPerDay); err != nil {
		return err
	}
	c.publishInvalidate(realmID, guildID)
	return nil
}

func (c *guildsCacheWithCoherence) DeleteLowestGuildRank(ctx context.Context, realmID uint32, guildID uint64, rank uint8) error {
	if err := c.GuildsCache.DeleteLowestGuildRank(ctx, realmID, guildID, rank); err != nil {
		return err
	}
	c.publishInvalidate(realmID, guildID)
	return nil
}
