package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/chatserver/repo"
	pbChat "github.com/walkline/ToCloud9/gen/chat/pb"
)

var (
	ErrPlayerBanned    = errors.New("player is banned")
	ErrWrongPassword   = errors.New("wrong password")
	ErrNotMember       = errors.New("not a member")
	ErrNotModerator    = errors.New("not a moderator")
	ErrNotOwner        = errors.New("not an owner")
	ErrPlayerNotFound  = errors.New("player not found in channel")
	ErrChannelNotFound = errors.New("channel not found")
	ErrMuted           = errors.New("player is muted")
)

const (
	// Member flags
	MemberFlagNone      uint8 = 0x00
	MemberFlagOwner     uint8 = 0x01
	MemberFlagModerator uint8 = 0x02
	MemberFlagVoiced    uint8 = 0x04
	MemberFlagMuted     uint8 = 0x08
	MemberFlagCustom    uint8 = 0x10
	MemberFlagMicMuted  uint8 = 0x20
)

const (
	// Channel flags
	ChannelFlagNone    uint8 = 0x00
	ChannelFlagCustom  uint8 = 0x01
	ChannelFlagTrade   uint8 = 0x04
	ChannelFlagNotLFG  uint8 = 0x08
	ChannelFlagGeneral uint8 = 0x10
	ChannelFlagCity    uint8 = 0x20
	ChannelFlagLFG     uint8 = 0x40
	ChannelFlagVoice   uint8 = 0x80
)

const (
	// Channel rights
	ChannelRightForceNoAnnouncements uint32 = 0x001
	ChannelRightForceAnnouncements   uint32 = 0x002
	ChannelRightNoOwnership          uint32 = 0x004
	ChannelRightCantSpeak            uint32 = 0x008
	ChannelRightCantBan              uint32 = 0x010
	ChannelRightCantKick             uint32 = 0x020
	ChannelRightCantMute             uint32 = 0x040
	ChannelRightCantChangePassword   uint32 = 0x080
	ChannelRightDontPreserve         uint32 = 0x100
)

// ActiveChannel represents an in-memory active channel
type ActiveChannel struct {
	mu sync.RWMutex

	channelID   uint32
	name        string
	team        pbChat.TeamID
	password    string
	announce    bool
	ownership   bool
	moderation  bool
	flags       uint8
	ownerGUID   uint64
	rights      *repo.ChannelRights
	members     map[uint64]*repo.ChannelMember
	bannedUntil map[uint64]time.Time
}

// ChannelManager manages active channels and their members
type ChannelManager struct {
	mu       sync.RWMutex
	channels map[string]*ActiveChannel // key: "realmID:channelName:team"
	repo     repo.ChannelsRepo
}

func NewChannelManager(channelsRepo repo.ChannelsRepo) *ChannelManager {
	return &ChannelManager{
		channels: make(map[string]*ActiveChannel),
		repo:     channelsRepo,
	}
}

func (cm *ChannelManager) channelKey(realmID uint32, channelName string, team pbChat.TeamID) string {
	return fmt.Sprintf("%d:%s:%d", realmID, strings.ToLower(channelName), int32(team))
}

// GetOrCreateChannel gets an existing channel or creates a new one
func (cm *ChannelManager) GetOrCreateChannel(ctx context.Context, realmID uint32, channelName string, channelID uint32, team pbChat.TeamID, password string, flags uint8) (*ActiveChannel, error) {
	key := cm.channelKey(realmID, channelName, team)

	cm.mu.RLock()
	ch, exists := cm.channels[key]
	cm.mu.RUnlock()

	if exists {
		return ch, nil
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Double-check after acquiring write lock
	if ch, exists := cm.channels[key]; exists {
		return ch, nil
	}

	// Try to load from database
	var dbChannel *repo.Channel
	var err error
	if channelID != 0 && flags&ChannelFlagCustom != 0 {
		dbChannel, err = cm.repo.GetChannelByID(ctx, realmID, channelID)
	} else {
		dbChannel, err = cm.repo.GetChannelByName(ctx, realmID, channelName, uint32(team))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get channel from DB: %w", err)
	}

	// Create new channel if not found in DB
	if dbChannel == nil {
		dbChannel = &repo.Channel{
			Name:      channelName,
			Team:      uint32(team),
			Announce:  true,
			Ownership: true,
			Password:  password,
			LastUsed:  time.Now(),
		}
		if err := cm.repo.CreateChannel(ctx, realmID, dbChannel); err != nil {
			return nil, fmt.Errorf("failed to create channel in DB: %w", err)
		}
	}

	// Load channel rights
	rights, err := cm.repo.GetChannelRights(ctx, realmID, channelName)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel rights: %w", err)
	}

	// Load bans
	bans, err := cm.repo.GetChannelBans(ctx, realmID, dbChannel.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel bans: %w", err)
	}

	bannedUntil := make(map[uint64]time.Time)
	for _, ban := range bans {
		if ban.BanTime.After(time.Now()) {
			bannedUntil[ban.PlayerGUID] = ban.BanTime
		}
	}

	// Load members
	loadedMembers, err := cm.repo.LoadChannelMembers(ctx, realmID, dbChannel.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("failed to load channel members: %w", err)
	}

	members := make(map[uint64]*repo.ChannelMember)
	var ownerGUID uint64
	for _, member := range loadedMembers {
		memberCopy := member // Create a copy to store pointer
		members[member.PlayerGUID] = &memberCopy
		if member.Flags&MemberFlagOwner != 0 {
			ownerGUID = member.PlayerGUID
		}
	}

	ch = &ActiveChannel{
		channelID:   dbChannel.ChannelID,
		name:        dbChannel.Name,
		team:        pbChat.TeamID(dbChannel.Team),
		password:    dbChannel.Password,
		announce:    dbChannel.Announce,
		ownership:   dbChannel.Ownership,
		moderation:  false,
		flags:       flags,
		rights:      rights,
		ownerGUID:   ownerGUID,
		members:     members,
		bannedUntil: bannedUntil,
	}

	cm.channels[key] = ch
	return ch, nil
}

// GetChannel retrieves an active channel if it exists
func (cm *ChannelManager) GetChannel(realmID uint32, channelName string, team pbChat.TeamID) *ActiveChannel {
	key := cm.channelKey(realmID, channelName, team)
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.channels[key]
}

// JoinChannel adds a player to a channel with write-through persistence
func (ch *ActiveChannel) JoinChannel(ctx context.Context, cm *ChannelManager, realmID uint32, playerGUID uint64, playerName string, password string) error {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	// Check if player is banned
	if banTime, banned := ch.bannedUntil[playerGUID]; banned && banTime.After(time.Now()) {
		return ErrPlayerBanned
	}

	// Check password if set
	if ch.password != "" && ch.password != password {
		return ErrWrongPassword
	}

	// Check if already a member
	if _, exists := ch.members[playerGUID]; exists {
		return nil // Already a member
	}

	// Determine initial flags
	flags := MemberFlagNone
	if len(ch.members) == 0 && ch.ownership {
		// First member becomes owner
		flags = MemberFlagOwner
		ch.ownerGUID = playerGUID
	}

	// Check if player is a moderator from rights
	if ch.rights != nil {
		for _, modGUID := range ch.rights.Moderators {
			if uint64(modGUID) == playerGUID {
				flags |= MemberFlagModerator
				break
			}
		}
	}

	member := &repo.ChannelMember{
		PlayerGUID: playerGUID,
		PlayerName: playerName,
		Flags:      flags,
	}

	ch.members[playerGUID] = member

	// Write through to DB
	if err := cm.repo.SaveChannelMember(ctx, realmID, ch.channelID, member); err != nil {
		// Remove from memory on DB failure
		delete(ch.members, playerGUID)
		return fmt.Errorf("failed to persist channel member: %w", err)
	}

	return nil
}

// LeaveChannel removes a player from a channel with write-through persistence
// Returns (newOwnerGUID, error) - newOwnerGUID is non-zero if ownership was transferred
func (ch *ActiveChannel) LeaveChannel(ctx context.Context, cm *ChannelManager, realmID uint32, playerGUID uint64) (uint64, error) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	member, exists := ch.members[playerGUID]
	if !exists {
		return 0, ErrNotMember
	}

	wasOwner := member.Flags&MemberFlagOwner != 0
	delete(ch.members, playerGUID)

	// Write through to DB
	if err := cm.repo.RemoveChannelMember(ctx, realmID, ch.channelID, playerGUID); err != nil {
		// Re-add to memory on DB failure
		ch.members[playerGUID] = member
		return 0, fmt.Errorf("failed to remove channel member from DB: %w", err)
	}

	var newOwnerGUID uint64

	// Transfer ownership if owner left
	if wasOwner && len(ch.members) > 0 && ch.ownership {
		// Find first moderator, or any member
		for guid, m := range ch.members {
			if m.Flags&MemberFlagModerator != 0 {
				newOwnerGUID = guid
				break
			}
		}
		if newOwnerGUID == 0 {
			// No moderator found, pick first member
			for guid := range ch.members {
				newOwnerGUID = guid
				break
			}
		}
		if newOwnerGUID != 0 {
			ch.members[newOwnerGUID].Flags |= MemberFlagOwner
			ch.ownerGUID = newOwnerGUID
			// Persist new owner flag
			if err := cm.repo.UpdateMemberFlags(ctx, realmID, ch.channelID, newOwnerGUID, ch.members[newOwnerGUID].Flags); err != nil {
				// Log but don't fail - ownership transfer is in-memory
				log.Error().Err(err).
					Uint64("newOwnerGUID", newOwnerGUID).
					Msg("Failed to persist owner flag transfer")
			}
		}
	}

	return newOwnerGUID, nil
}

// GetMembers returns a copy of all channel members
func (ch *ActiveChannel) GetMembers() []repo.ChannelMember {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	members := make([]repo.ChannelMember, 0, len(ch.members))
	for _, m := range ch.members {
		members = append(members, *m)
	}
	return members
}

// IsMember checks if a player is a member
func (ch *ActiveChannel) IsMember(playerGUID uint64) bool {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	_, exists := ch.members[playerGUID]
	return exists
}

// GetMemberFlags gets the flags for a member
func (ch *ActiveChannel) GetMemberFlags(playerGUID uint64) uint8 {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	if member, exists := ch.members[playerGUID]; exists {
		return member.Flags
	}
	return MemberFlagNone
}

// FindMemberByName returns the GUID of a member with the given name, or 0 if not found
func (ch *ActiveChannel) FindMemberByName(name string) uint64 {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	for _, m := range ch.members {
		if m.PlayerName == name {
			return m.PlayerGUID
		}
	}
	return 0
}

// SetModerator sets or unsets moderator flag for a player with write-through persistence
func (ch *ActiveChannel) SetModerator(ctx context.Context, cm *ChannelManager, realmID uint32, playerGUID uint64, isModerator bool) error {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	member, exists := ch.members[playerGUID]
	if !exists {
		return ErrNotMember
	}

	oldFlags := member.Flags
	if isModerator {
		member.Flags |= MemberFlagModerator
	} else {
		member.Flags &= ^MemberFlagModerator
	}

	// Write through to DB
	if err := cm.repo.UpdateMemberFlags(ctx, realmID, ch.channelID, playerGUID, member.Flags); err != nil {
		// Revert on failure
		member.Flags = oldFlags
		return fmt.Errorf("failed to persist moderator flag: %w", err)
	}

	return nil
}

// SetMute sets or unsets mute flag for a player with write-through persistence
func (ch *ActiveChannel) SetMute(ctx context.Context, cm *ChannelManager, realmID uint32, playerGUID uint64, isMuted bool) error {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	member, exists := ch.members[playerGUID]
	if !exists {
		return ErrNotMember
	}

	oldFlags := member.Flags
	if isMuted {
		member.Flags |= MemberFlagMuted
	} else {
		member.Flags &= ^MemberFlagMuted
	}

	// Write through to DB
	if err := cm.repo.UpdateMemberFlags(ctx, realmID, ch.channelID, playerGUID, member.Flags); err != nil {
		// Revert on failure
		member.Flags = oldFlags
		return fmt.Errorf("failed to persist mute flag: %w", err)
	}

	return nil
}

// GetChannelID returns the channel ID
func (ch *ActiveChannel) GetChannelID() uint32 {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return ch.channelID
}

// GetName returns the channel name
func (ch *ActiveChannel) GetName() string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return ch.name
}

// GetFlags returns the channel flags
func (ch *ActiveChannel) GetFlags() uint8 {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return ch.flags
}

// GetNumMembers returns the number of members
func (ch *ActiveChannel) GetNumMembers() int {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return len(ch.members)
}

// ToggleModeration toggles the moderation state
func (ch *ActiveChannel) ToggleModeration() bool {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.moderation = !ch.moderation
	return ch.moderation
}

// ToggleAnnouncements toggles the announcements state
func (ch *ActiveChannel) ToggleAnnouncements() bool {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.announce = !ch.announce
	return ch.announce
}

// CanSpeak checks if a player can send messages
func (ch *ActiveChannel) CanSpeak(playerGUID uint64) bool {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	member, exists := ch.members[playerGUID]
	if !exists {
		return false
	}

	// Check if muted
	if member.Flags&MemberFlagMuted != 0 {
		return false
	}

	// Check if moderation is enabled and player is not voiced/moderator/owner
	if ch.moderation {
		if member.Flags&(MemberFlagVoiced|MemberFlagModerator|MemberFlagOwner) == 0 {
			return false
		}
	}

	// Check rights
	if ch.rights != nil && ch.rights.Flags&ChannelRightCantSpeak != 0 {
		return false
	}

	return true
}

// BanPlayer bans a player from the channel with write-through to DB
func (cm *ChannelManager) BanPlayer(ctx context.Context, realmID uint32, channelName string, team pbChat.TeamID, playerGUID uint64, banTime time.Time) error {
	ch := cm.GetChannel(realmID, channelName, team)
	if ch == nil {
		return ErrChannelNotFound
	}

	ch.mu.Lock()
	ch.bannedUntil[playerGUID] = banTime
	ch.mu.Unlock()

	// Write through to DB
	ban := &repo.ChannelBan{
		ChannelID:  ch.channelID,
		PlayerGUID: playerGUID,
		BanTime:    banTime,
	}
	return cm.repo.AddChannelBan(ctx, realmID, ban)
}

// UnbanPlayer removes a ban from the channel with write-through to DB
func (cm *ChannelManager) UnbanPlayer(ctx context.Context, realmID uint32, channelName string, team pbChat.TeamID, playerGUID uint64) error {
	ch := cm.GetChannel(realmID, channelName, team)
	if ch == nil {
		return ErrChannelNotFound
	}

	ch.mu.Lock()
	delete(ch.bannedUntil, playerGUID)
	channelID := ch.channelID
	ch.mu.Unlock()

	// Write through to DB
	return cm.repo.RemoveChannelBan(ctx, realmID, channelID, playerGUID)
}

// SetPassword sets the channel password with write-through to DB
func (ch *ActiveChannel) SetPassword(ctx context.Context, cm *ChannelManager, realmID uint32, password string) error {
	ch.mu.Lock()
	ch.password = password
	dbChannel := &repo.Channel{
		ChannelID: ch.channelID,
		Name:      ch.name,
		Team:      uint32(ch.team),
		Announce:  ch.announce,
		Ownership: ch.ownership,
		Password:  password,
		LastUsed:  time.Now(),
	}
	ch.mu.Unlock()

	// Write through to DB
	return cm.repo.UpdateChannel(ctx, realmID, dbChannel)
}

// SetOwner transfers ownership to another player with write-through persistence
func (ch *ActiveChannel) SetOwner(ctx context.Context, cm *ChannelManager, realmID uint32, newOwnerGUID uint64) error {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	newOwner, exists := ch.members[newOwnerGUID]
	if !exists {
		return ErrPlayerNotFound
	}

	oldOwnerGUID := ch.ownerGUID
	var oldOwnerOldFlags uint8

	// Remove owner flag from old owner
	if ch.ownerGUID != 0 {
		if oldOwner, exists := ch.members[ch.ownerGUID]; exists {
			oldOwnerOldFlags = oldOwner.Flags
			oldOwner.Flags &= ^MemberFlagOwner
			// Persist old owner flag change
			if err := cm.repo.UpdateMemberFlags(ctx, realmID, ch.channelID, ch.ownerGUID, oldOwner.Flags); err != nil {
				// Revert
				oldOwner.Flags = oldOwnerOldFlags
				return fmt.Errorf("failed to persist old owner flag removal: %w", err)
			}
		}
	}

	// Set new owner
	newOwnerOldFlags := newOwner.Flags
	newOwner.Flags |= MemberFlagOwner

	// Persist new owner flag
	if err := cm.repo.UpdateMemberFlags(ctx, realmID, ch.channelID, newOwnerGUID, newOwner.Flags); err != nil {
		// Revert both changes
		newOwner.Flags = newOwnerOldFlags
		if oldOwnerGUID != 0 {
			if oldOwner, exists := ch.members[oldOwnerGUID]; exists {
				oldOwner.Flags = oldOwnerOldFlags
			}
		}
		return fmt.Errorf("failed to persist new owner flag: %w", err)
	}

	ch.ownerGUID = newOwnerGUID
	return nil
}

// UpdateLastUsed updates the lastUsed timestamp in DB
func (cm *ChannelManager) UpdateLastUsed(ctx context.Context, realmID uint32, channelName string, team pbChat.TeamID) error {
	ch := cm.GetChannel(realmID, channelName, team)
	if ch == nil {
		return nil
	}

	ch.mu.RLock()
	channelID := ch.channelID
	ch.mu.RUnlock()

	if channelID == 0 {
		return nil
	}

	return cm.repo.UpdateChannelLastUsed(ctx, realmID, channelID)
}

// PersistToggleAnnouncements persists the announce state to DB
func (ch *ActiveChannel) PersistToggleAnnouncements(ctx context.Context, cm *ChannelManager, realmID uint32) error {
	ch.mu.RLock()
	dbChannel := &repo.Channel{
		ChannelID: ch.channelID,
		Name:      ch.name,
		Team:      uint32(ch.team),
		Announce:  ch.announce,
		Ownership: ch.ownership,
		Password:  ch.password,
		LastUsed:  time.Now(),
	}
	ch.mu.RUnlock()

	return cm.repo.UpdateChannel(ctx, realmID, dbChannel)
}

// High-level service methods that combine permission checks, name resolution, and business logic

// KickPlayer kicks a player from a channel after checking permissions.
// Returns the channel and target GUID for broadcasting.
func (cm *ChannelManager) KickPlayer(ctx context.Context, realmID uint32, channelName string, team pbChat.TeamID, kickerGUID uint64, targetName string) (*ActiveChannel, uint64, error) {
	channel := cm.GetChannel(realmID, channelName, team)
	if channel == nil {
		return nil, 0, ErrChannelNotFound
	}

	if !channel.IsMember(kickerGUID) {
		return nil, 0, ErrNotMember
	}

	kickerFlags := channel.GetMemberFlags(kickerGUID)
	if kickerFlags&(MemberFlagModerator|MemberFlagOwner) == 0 {
		return nil, 0, ErrNotModerator
	}

	targetGUID := channel.FindMemberByName(targetName)
	if targetGUID == 0 {
		return nil, 0, ErrPlayerNotFound
	}

	newOwnerGUID, err := channel.LeaveChannel(ctx, cm, realmID, targetGUID)
	if err != nil {
		return nil, 0, err
	}

	// If ownership was transferred, we need to notify (caller should broadcast)
	_ = newOwnerGUID // Caller will handle broadcasting

	return channel, targetGUID, nil
}

// BanPlayerByName kicks and bans a player, persisting the ban to DB.
func (cm *ChannelManager) BanPlayerByName(ctx context.Context, realmID uint32, channelName string, team pbChat.TeamID, bannerGUID uint64, targetName string) (*ActiveChannel, uint64, error) {
	channel := cm.GetChannel(realmID, channelName, team)
	if channel == nil {
		return nil, 0, ErrChannelNotFound
	}

	if !channel.IsMember(bannerGUID) {
		return nil, 0, ErrNotMember
	}

	bannerFlags := channel.GetMemberFlags(bannerGUID)
	if bannerFlags&(MemberFlagModerator|MemberFlagOwner) == 0 {
		return nil, 0, ErrNotModerator
	}

	targetGUID := channel.FindMemberByName(targetName)
	if targetGUID == 0 {
		return nil, 0, ErrPlayerNotFound
	}

	// Kick from channel
	if _, err := channel.LeaveChannel(ctx, cm, realmID, targetGUID); err != nil {
		// Log but don't fail if already left
		log.Debug().Err(err).Msg("Failed to kick player during ban (already left?)")
	}

	// Add permanent ban (100 years)
	banTime := time.Now().Add(100 * 365 * 24 * time.Hour)
	if err := cm.BanPlayer(ctx, realmID, channelName, team, targetGUID, banTime); err != nil {
		return nil, 0, fmt.Errorf("failed to persist ban: %w", err)
	}

	return channel, targetGUID, nil
}

// UnbanPlayerByName removes a ban after checking permissions.
func (cm *ChannelManager) UnbanPlayerByName(ctx context.Context, realmID uint32, channelName string, team pbChat.TeamID, unbannerGUID uint64, targetName string) (uint64, error) {
	channel := cm.GetChannel(realmID, channelName, team)
	if channel == nil {
		return 0, ErrChannelNotFound
	}

	// Note: unbanner doesn't need to be a member to unban
	// Check permissions
	unbannerFlags := channel.GetMemberFlags(unbannerGUID)
	if unbannerFlags&(MemberFlagModerator|MemberFlagOwner) == 0 {
		return 0, ErrNotModerator
	}

	// Find target - this looks through current members, which is awkward for banned players
	// The original server code has the same limitation
	targetGUID := channel.FindMemberByName(targetName)
	if targetGUID == 0 {
		return 0, ErrPlayerNotFound
	}

	if err := cm.UnbanPlayer(ctx, realmID, channelName, team, targetGUID); err != nil {
		return 0, err
	}

	return targetGUID, nil
}

// SetModeratorByName sets/unsets moderator for a player found by name. Requires owner.
func (cm *ChannelManager) SetModeratorByName(ctx context.Context, realmID uint32, channelName string, team pbChat.TeamID, setterGUID uint64, targetName string, isModerator bool) (uint64, error) {
	channel := cm.GetChannel(realmID, channelName, team)
	if channel == nil {
		return 0, ErrChannelNotFound
	}

	if !channel.IsMember(setterGUID) {
		return 0, ErrNotMember
	}

	setterFlags := channel.GetMemberFlags(setterGUID)
	if setterFlags&MemberFlagOwner == 0 {
		return 0, ErrNotOwner
	}

	targetGUID := channel.FindMemberByName(targetName)
	if targetGUID == 0 {
		return 0, ErrPlayerNotFound
	}

	if err := channel.SetModerator(ctx, cm, realmID, targetGUID, isModerator); err != nil {
		return 0, err
	}

	return targetGUID, nil
}

// SetMuteByName sets/unsets mute for a player found by name. Requires moderator or owner.
func (cm *ChannelManager) SetMuteByName(ctx context.Context, realmID uint32, channelName string, team pbChat.TeamID, muterGUID uint64, targetName string, isMuted bool) (uint64, error) {
	channel := cm.GetChannel(realmID, channelName, team)
	if channel == nil {
		return 0, ErrChannelNotFound
	}

	if !channel.IsMember(muterGUID) {
		return 0, ErrNotMember
	}

	muterFlags := channel.GetMemberFlags(muterGUID)
	if muterFlags&(MemberFlagModerator|MemberFlagOwner) == 0 {
		return 0, ErrNotModerator
	}

	targetGUID := channel.FindMemberByName(targetName)
	if targetGUID == 0 {
		return 0, ErrPlayerNotFound
	}

	if err := channel.SetMute(ctx, cm, realmID, targetGUID, isMuted); err != nil {
		return 0, err
	}

	return targetGUID, nil
}

// SetOwnerByName transfers ownership. Requires current owner.
func (cm *ChannelManager) SetOwnerByName(ctx context.Context, realmID uint32, channelName string, team pbChat.TeamID, setterGUID uint64, targetName string) (uint64, error) {
	channel := cm.GetChannel(realmID, channelName, team)
	if channel == nil {
		return 0, ErrChannelNotFound
	}

	setterFlags := channel.GetMemberFlags(setterGUID)
	if setterFlags&MemberFlagOwner == 0 {
		return 0, ErrNotOwner
	}

	targetGUID := channel.FindMemberByName(targetName)
	if targetGUID == 0 {
		return 0, ErrPlayerNotFound
	}

	if err := channel.SetOwner(ctx, cm, realmID, targetGUID); err != nil {
		return 0, err
	}

	return targetGUID, nil
}

// SetChannelPassword sets the password. Requires owner. Persists to DB.
func (cm *ChannelManager) SetChannelPassword(ctx context.Context, realmID uint32, channelName string, team pbChat.TeamID, setterGUID uint64, password string) error {
	channel := cm.GetChannel(realmID, channelName, team)
	if channel == nil {
		return ErrChannelNotFound
	}

	setterFlags := channel.GetMemberFlags(setterGUID)
	if setterFlags&MemberFlagOwner == 0 {
		return ErrNotOwner
	}

	channel.mu.Lock()
	channel.password = password
	dbChannel := &repo.Channel{
		ChannelID: channel.channelID,
		Name:      channel.name,
		Team:      uint32(channel.team),
		Announce:  channel.announce,
		Ownership: channel.ownership,
		Password:  password,
		LastUsed:  time.Now(),
	}
	channel.mu.Unlock()

	return cm.repo.UpdateChannel(ctx, realmID, dbChannel)
}

// ToggleChannelModeration toggles moderation. Requires moderator or owner.
func (cm *ChannelManager) ToggleChannelModeration(realmID uint32, channelName string, team pbChat.TeamID, togglerGUID uint64) (bool, error) {
	channel := cm.GetChannel(realmID, channelName, team)
	if channel == nil {
		return false, ErrChannelNotFound
	}

	if !channel.IsMember(togglerGUID) {
		return false, ErrNotMember
	}

	togglerFlags := channel.GetMemberFlags(togglerGUID)
	if togglerFlags&(MemberFlagModerator|MemberFlagOwner) == 0 {
		return false, ErrNotModerator
	}

	enabled := channel.ToggleModeration()
	return enabled, nil
}

// ToggleChannelAnnouncements toggles announcements. Requires moderator or owner. Persists to DB.
func (cm *ChannelManager) ToggleChannelAnnouncements(ctx context.Context, realmID uint32, channelName string, team pbChat.TeamID, togglerGUID uint64) (bool, error) {
	channel := cm.GetChannel(realmID, channelName, team)
	if channel == nil {
		return false, ErrChannelNotFound
	}

	if !channel.IsMember(togglerGUID) {
		return false, ErrNotMember
	}

	togglerFlags := channel.GetMemberFlags(togglerGUID)
	if togglerFlags&(MemberFlagModerator|MemberFlagOwner) == 0 {
		return false, ErrNotModerator
	}

	enabled := channel.ToggleAnnouncements()

	// Persist to DB
	channel.mu.RLock()
	dbChannel := &repo.Channel{
		ChannelID: channel.channelID,
		Name:      channel.name,
		Team:      uint32(channel.team),
		Announce:  channel.announce,
		Ownership: channel.ownership,
		Password:  channel.password,
		LastUsed:  time.Now(),
	}
	channel.mu.RUnlock()

	if err := cm.repo.UpdateChannel(ctx, realmID, dbChannel); err != nil {
		return enabled, fmt.Errorf("failed to persist announcement toggle: %w", err)
	}

	return enabled, nil
}

// LeaveChannelByGUID handles a player leaving a channel. Returns player name, whether channel is custom, and new owner GUID if transferred.
func (cm *ChannelManager) LeaveChannelByGUID(ctx context.Context, realmID uint32, channelName string, team pbChat.TeamID, playerGUID uint64) (string, bool, uint64, error) {
	channel := cm.GetChannel(realmID, channelName, team)
	if channel == nil {
		return "", false, 0, ErrChannelNotFound
	}

	// Find player name before leaving
	var playerName string
	members := channel.GetMembers()
	for _, m := range members {
		if m.PlayerGUID == playerGUID {
			playerName = m.PlayerName
			break
		}
	}

	newOwnerGUID, err := channel.LeaveChannel(ctx, cm, realmID, playerGUID)
	if err != nil {
		return "", false, 0, err
	}

	isCustom := channel.GetFlags()&ChannelFlagCustom != 0
	return playerName, isCustom, newOwnerGUID, nil
}

// ValidateSendMessage validates that a player can send a message. Returns channel for broadcasting.
func (cm *ChannelManager) ValidateSendMessage(realmID uint32, channelName string, team pbChat.TeamID, senderGUID uint64) (*ActiveChannel, error) {
	channel := cm.GetChannel(realmID, channelName, team)
	if channel == nil {
		return nil, ErrNotMember
	}

	if !channel.IsMember(senderGUID) {
		return nil, ErrNotMember
	}

	if !channel.CanSpeak(senderGUID) {
		return nil, ErrMuted
	}

	return channel, nil
}

// OwnershipTransfer represents an ownership change in a channel
type OwnershipTransfer struct {
	ChannelName  string
	ChannelID    uint32
	TeamID       pbChat.TeamID
	NewOwnerGUID uint64
}

// TransferOwnershipOnLogout transfers channel ownership when owner logs out (but keeps them as member)
// Returns list of ownership transfers that occurred
func (cm *ChannelManager) TransferOwnershipOnLogout(realmID uint32, playerGUID uint64) []OwnershipTransfer {
	ctx := context.Background() // Background context for logout operations
	var transfers []OwnershipTransfer

	// First pass: collect channels to process (read lock only)
	// Note: We collect *pointers* to ActiveChannel objects. Even if cleanup concurrently
	// removes a channel from the map, the channel object remains valid in memory (Go GC
	// won't collect it while we hold a reference). Each channel has its own mutex for
	// thread-safe access to members.
	cm.mu.RLock()
	channelsToProcess := make([]*ActiveChannel, 0)
	for key, channel := range cm.channels {
		if strings.HasPrefix(key, fmt.Sprintf("%d:", realmID)) {
			channelsToProcess = append(channelsToProcess, channel)
		}
	}
	cm.mu.RUnlock()

	// Second pass: process each channel - transfer ownership if needed
	for _, channel := range channelsToProcess {
		channel.mu.Lock()
		member, isMember := channel.members[playerGUID]
		if !isMember {
			channel.mu.Unlock()
			continue
		}

		// Check if this member is the owner
		isOwner := member.Flags&MemberFlagOwner != 0
		if !isOwner || !channel.ownership {
			channel.mu.Unlock()
			continue
		}

		// Owner is logging out - find another ONLINE member to transfer to
		// We need to find someone else who is still in the members list
		var newOwnerGUID uint64
		for guid, m := range channel.members {
			if guid == playerGUID {
				continue // Skip the player logging out
			}
			// Prefer moderators
			if m.Flags&MemberFlagModerator != 0 {
				newOwnerGUID = guid
				break
			}
		}

		if newOwnerGUID == 0 {
			// No moderator found, pick any other member (not the one logging out)
			for guid := range channel.members {
				if guid != playerGUID {
					newOwnerGUID = guid
					break
				}
			}
		}

		if newOwnerGUID != 0 {
			// Remove owner flag from logging out player
			member.Flags &= ^MemberFlagOwner
			if err := cm.repo.UpdateMemberFlags(ctx, realmID, channel.channelID, playerGUID, member.Flags); err != nil {
				log.Error().Err(err).
					Uint64("playerGUID", playerGUID).
					Uint32("channelID", channel.channelID).
					Msg("Failed to remove owner flag on logout")
			}

			// Transfer ownership to new owner
			channel.members[newOwnerGUID].Flags |= MemberFlagOwner
			channel.ownerGUID = newOwnerGUID

			// Persist new owner flag
			if err := cm.repo.UpdateMemberFlags(ctx, realmID, channel.channelID, newOwnerGUID, channel.members[newOwnerGUID].Flags); err != nil {
				log.Error().Err(err).
					Uint64("newOwnerGUID", newOwnerGUID).
					Uint32("channelID", channel.channelID).
					Msg("Failed to persist owner transfer on logout")
			}

			// Record ownership transfer
			transfers = append(transfers, OwnershipTransfer{
				ChannelName:  channel.name,
				ChannelID:    channel.channelID,
				TeamID:       channel.team,
				NewOwnerGUID: newOwnerGUID,
			})

			log.Debug().
				Uint64("oldOwnerGUID", playerGUID).
				Uint64("newOwnerGUID", newOwnerGUID).
				Str("channelName", channel.name).
				Msg("Transferred channel ownership on logout (player remains member)")
		} else {
			// No other members - player remains owner even while offline
			log.Debug().
				Uint64("ownerGUID", playerGUID).
				Str("channelName", channel.name).
				Msg("Owner logged out but no other members - remains owner")
		}

		channel.mu.Unlock()
	}

	return transfers
}

// PreloadChannels loads all channels for a realm from DB into memory
func (cm *ChannelManager) PreloadChannels(ctx context.Context, realmID uint32) error {
	// Get all channels from DB
	dbChannels, err := cm.repo.GetAllChannels(ctx, realmID)
	if err != nil {
		return fmt.Errorf("failed to load channels from DB: %w", err)
	}

	log.Info().Uint32("realmID", realmID).Int("channelCount", len(dbChannels)).Msg("Preloading channels...")

	// Load each channel
	for _, dbChannel := range dbChannels {
		// Use GetOrCreateChannel to properly initialize the channel with all data
		_, err := cm.GetOrCreateChannel(
			ctx,
			realmID,
			dbChannel.Name,
			dbChannel.ChannelID,
			pbChat.TeamID(dbChannel.Team),
			"", // Password is loaded from DB
			ChannelFlagCustom,
		)
		if err != nil {
			log.Error().Err(err).
				Str("channelName", dbChannel.Name).
				Uint32("channelID", dbChannel.ChannelID).
				Msg("Failed to preload channel")
			continue
		}
	}

	log.Info().Uint32("realmID", realmID).Int("loaded", len(dbChannels)).Msg("Channels preloaded successfully")
	return nil
}

// PruneOfflineMembersFromAllChannels validates and removes offline members from all loaded channels
func (cm *ChannelManager) PruneOfflineMembersFromAllChannels(ctx context.Context, realmID uint32, onlineGUIDs map[uint64]bool) error {
	// Collect channel pointers under read lock
	// Safe: Even if cleanup removes channels from map concurrently, the channel objects
	// remain valid (we hold pointers). Each channel's own mutex protects member access.
	cm.mu.RLock()
	channels := make([]*ActiveChannel, 0, len(cm.channels))
	for key, ch := range cm.channels {
		if strings.HasPrefix(key, fmt.Sprintf("%d:", realmID)) {
			channels = append(channels, ch)
		}
	}
	cm.mu.RUnlock()

	for _, ch := range channels {
		if err := ch.PruneOfflineMembers(ctx, cm, realmID, onlineGUIDs); err != nil {
			log.Error().Err(err).
				Str("channelName", ch.GetName()).
				Uint32("channelID", ch.GetChannelID()).
				Msg("Failed to prune offline members from channel")
		}
	}

	return nil
}

// CleanupStaleChannels removes empty channels from memory and cleans old data from DB
// This method is idempotent and safe to run concurrently from multiple instances.
// Memory cleanup is local to each instance, DB operations use idempotent DELETE queries.
func (cm *ChannelManager) CleanupStaleChannels(ctx context.Context, realmID uint32, inactiveDuration time.Duration) error {
	log.Info().
		Uint32("realmID", realmID).
		Dur("inactiveDuration", inactiveDuration).
		Msg("Starting channel cleanup")

	// Memory cleanup - local to this instance, no conflicts with other instances
	removedCount := cm.cleanupLocalMemory(realmID)

	log.Info().
		Int("removedFromMemory", removedCount).
		Msg("Removed empty channels from local memory")

	// DB cleanup - idempotent operations (DELETE WHERE is safe to run multiple times)
	// Multiple instances may execute these simultaneously, but that's okay:
	// - DELETE queries are idempotent (deleting already-deleted rows is a no-op)
	// - Worst case: duplicate work, but no data corruption

	cutoffTime := time.Now().Add(-inactiveDuration)
	if err := cm.repo.CleanOldChannels(ctx, realmID, cutoffTime); err != nil {
		log.Error().Err(err).Msg("Failed to clean old channels from DB")
		return fmt.Errorf("failed to clean old channels: %w", err)
	}

	if err := cm.repo.CleanExpiredBans(ctx, realmID); err != nil {
		log.Error().Err(err).Msg("Failed to clean expired bans from DB")
		return fmt.Errorf("failed to clean expired bans: %w", err)
	}

	log.Info().Msg("Channel cleanup completed successfully")
	return nil
}

// cleanupLocalMemory removes empty channels from this instance's memory only
func (cm *ChannelManager) cleanupLocalMemory(realmID uint32) int {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	keysToRemove := make([]string, 0)
	realmPrefix := fmt.Sprintf("%d:", realmID)

	for key, ch := range cm.channels {
		// Only process channels for this realm
		if !strings.HasPrefix(key, realmPrefix) {
			continue
		}

		// Remove empty channels from memory
		if ch.GetNumMembers() == 0 {
			keysToRemove = append(keysToRemove, key)
		}
	}

	for _, key := range keysToRemove {
		delete(cm.channels, key)
	}

	return len(keysToRemove)
}

// PruneOfflineMembers removes offline members from the channel
func (ch *ActiveChannel) PruneOfflineMembers(ctx context.Context, cm *ChannelManager, realmID uint32, onlineGUIDs map[uint64]bool) error {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	offlineGUIDs := make([]uint64, 0)
	for guid := range ch.members {
		if !onlineGUIDs[guid] {
			offlineGUIDs = append(offlineGUIDs, guid)
		}
	}

	// Remove offline members
	for _, guid := range offlineGUIDs {
		delete(ch.members, guid)
		// Remove from DB
		if err := cm.repo.RemoveChannelMember(ctx, realmID, ch.channelID, guid); err != nil {
			log.Error().Err(err).
				Uint64("playerGUID", guid).
				Uint32("channelID", ch.channelID).
				Msg("Failed to remove offline member from DB")
		}
	}

	// Handle owner transfer if owner is offline
	if ch.ownerGUID != 0 && !onlineGUIDs[ch.ownerGUID] {
		ch.ownerGUID = 0
		// Find new owner among remaining members
		if len(ch.members) > 0 && ch.ownership {
			// Find first moderator, or any member
			var newOwnerGUID uint64
			for guid, m := range ch.members {
				if m.Flags&MemberFlagModerator != 0 {
					newOwnerGUID = guid
					break
				}
			}
			if newOwnerGUID == 0 {
				// No moderator found, pick first member
				for guid := range ch.members {
					newOwnerGUID = guid
					break
				}
			}
			if newOwnerGUID != 0 {
				ch.members[newOwnerGUID].Flags |= MemberFlagOwner
				ch.ownerGUID = newOwnerGUID
				// Persist new owner flag
				if err := cm.repo.UpdateMemberFlags(ctx, realmID, ch.channelID, newOwnerGUID, ch.members[newOwnerGUID].Flags); err != nil {
					log.Error().Err(err).
						Uint64("newOwnerGUID", newOwnerGUID).
						Msg("Failed to persist new owner flag")
				}
			}
		}
	}

	return nil
}
