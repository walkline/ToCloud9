package session

import (
	"context"

	root "github.com/walkline/ToCloud9/apps/gateway"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	pbChat "github.com/walkline/ToCloud9/gen/chat/pb"
)

// HandleChannelPassword handles CMSG_CHANNEL_PASSWORD - set channel password
func (s *GameSession) HandleChannelPassword(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	channelName := r.String()
	if s.forwardNativeChannelPacket(channelName, p) {
		return nil
	}
	channelRealmID, channelTeamID, servicePlayerGUID := s.channelScopeForName(channelName)
	password := r.String()

	resp, err := s.chatServiceClient.SetChannelPassword(ctx, &pbChat.SetChannelPasswordRequest{
		Api:         root.Ver,
		RealmID:     channelRealmID,
		SetterGUID:  servicePlayerGUID,
		ChannelName: channelName,
		Password:    password,
		TeamID:      channelTeamID,
	})
	if err != nil {
		s.logger.Error().Err(err).Str("channelName", channelName).Msg("Failed to set channel password")
		return err
	}

	// Get channel info for notifications
	ch := s.channelMembership.GetChannel(channelName)
	if ch == nil {
		return nil
	}

	switch resp.Status {
	case pbChat.SetChannelPasswordResponse_Ok:
		s.ChannelNotify(ch).PlayerAction(ChatPasswordChangedNotice, s.character.GUID)
	case pbChat.SetChannelPasswordResponse_NotMember:
		s.ChannelNotify(ch).Simple(ChatNotMemberNotice)
	case pbChat.SetChannelPasswordResponse_NotOwner:
		s.ChannelNotify(ch).Simple(ChatNotOwnerNotice)
	}

	return nil
}

// HandleChannelSetOwner handles CMSG_CHANNEL_SET_OWNER - transfer channel ownership
func (s *GameSession) HandleChannelSetOwner(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	channelName := r.String()
	if s.forwardNativeChannelPacket(channelName, p) {
		return nil
	}
	channelRealmID, channelTeamID, servicePlayerGUID := s.channelScopeForName(channelName)
	targetName := r.String()

	resp, err := s.chatServiceClient.SetChannelOwner(ctx, &pbChat.SetChannelOwnerRequest{
		Api:         root.Ver,
		RealmID:     channelRealmID,
		SetterGUID:  servicePlayerGUID,
		ChannelName: channelName,
		TargetName:  targetName,
		TeamID:      channelTeamID,
	})
	if err != nil {
		s.logger.Error().Err(err).Str("channelName", channelName).Str("targetName", targetName).Msg("Failed to set channel owner")
		return err
	}

	ch := s.channelMembership.GetChannel(channelName)
	if ch == nil {
		return nil
	}

	switch resp.Status {
	case pbChat.SetChannelOwnerResponse_Ok:
		// Chatserver broadcasts AzerothCore-shaped mode/owner notifications with the target GUID.
	case pbChat.SetChannelOwnerResponse_NotMember:
		s.ChannelNotify(ch).Simple(ChatNotMemberNotice)
	case pbChat.SetChannelOwnerResponse_NotOwner:
		s.ChannelNotify(ch).Simple(ChatNotOwnerNotice)
	case pbChat.SetChannelOwnerResponse_PlayerNotFound:
		s.ChannelNotify(ch).PlayerName(ChatPlayerNotFoundNotice, targetName)
	}

	return nil
}

// HandleChannelSetModerator handles CMSG_CHANNEL_MODERATOR - grant moderator status
func (s *GameSession) HandleChannelSetModerator(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	channelName := r.String()
	if s.forwardNativeChannelPacket(channelName, p) {
		return nil
	}
	channelRealmID, channelTeamID, servicePlayerGUID := s.channelScopeForName(channelName)
	targetName := r.String()

	resp, err := s.chatServiceClient.SetChannelModerator(ctx, &pbChat.SetChannelModeratorRequest{
		Api:         root.Ver,
		RealmID:     channelRealmID,
		SetterGUID:  servicePlayerGUID,
		ChannelName: channelName,
		TargetName:  targetName,
		TeamID:      channelTeamID,
	})
	if err != nil {
		s.logger.Error().Err(err).Str("channelName", channelName).Str("targetName", targetName).Msg("Failed to set channel moderator")
		return err
	}

	ch := s.channelMembership.GetChannel(channelName)
	if ch == nil {
		return nil
	}

	switch resp.Status {
	case pbChat.SetChannelModeratorResponse_Ok:
		// Chatserver broadcasts the AzerothCore-shaped mode-change notification with the target GUID.
	case pbChat.SetChannelModeratorResponse_NotMember:
		s.ChannelNotify(ch).Simple(ChatNotMemberNotice)
	case pbChat.SetChannelModeratorResponse_NotOwner:
		s.ChannelNotify(ch).Simple(ChatNotOwnerNotice)
	case pbChat.SetChannelModeratorResponse_PlayerNotFound:
		s.ChannelNotify(ch).PlayerName(ChatPlayerNotFoundNotice, targetName)
	}

	return nil
}

// HandleChannelUnsetModerator handles CMSG_CHANNEL_UN_MODERATOR - remove moderator status
func (s *GameSession) HandleChannelUnsetModerator(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	channelName := r.String()
	if s.forwardNativeChannelPacket(channelName, p) {
		return nil
	}
	channelRealmID, channelTeamID, servicePlayerGUID := s.channelScopeForName(channelName)
	targetName := r.String()

	resp, err := s.chatServiceClient.UnsetChannelModerator(ctx, &pbChat.UnsetChannelModeratorRequest{
		Api:         root.Ver,
		RealmID:     channelRealmID,
		SetterGUID:  servicePlayerGUID,
		ChannelName: channelName,
		TargetName:  targetName,
		TeamID:      channelTeamID,
	})
	if err != nil {
		s.logger.Error().Err(err).Str("channelName", channelName).Str("targetName", targetName).Msg("Failed to unset channel moderator")
		return err
	}

	ch := s.channelMembership.GetChannel(channelName)
	if ch == nil {
		return nil
	}

	switch resp.Status {
	case pbChat.UnsetChannelModeratorResponse_Ok:
		// Chatserver broadcasts the AzerothCore-shaped mode-change notification with the target GUID.
	case pbChat.UnsetChannelModeratorResponse_NotMember:
		s.ChannelNotify(ch).Simple(ChatNotMemberNotice)
	case pbChat.UnsetChannelModeratorResponse_NotOwner:
		s.ChannelNotify(ch).Simple(ChatNotOwnerNotice)
	case pbChat.UnsetChannelModeratorResponse_PlayerNotFound:
		s.ChannelNotify(ch).PlayerName(ChatPlayerNotFoundNotice, targetName)
	}

	return nil
}

// HandleChannelMute handles CMSG_CHANNEL_MUTE - mute a player
func (s *GameSession) HandleChannelMute(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	channelName := r.String()
	if s.forwardNativeChannelPacket(channelName, p) {
		return nil
	}
	channelRealmID, channelTeamID, servicePlayerGUID := s.channelScopeForName(channelName)
	targetName := r.String()

	resp, err := s.chatServiceClient.SetChannelMute(ctx, &pbChat.SetChannelMuteRequest{
		Api:         root.Ver,
		RealmID:     channelRealmID,
		MuterGUID:   servicePlayerGUID,
		ChannelName: channelName,
		TargetName:  targetName,
		TeamID:      channelTeamID,
	})
	if err != nil {
		s.logger.Error().Err(err).Str("channelName", channelName).Str("targetName", targetName).Msg("Failed to mute player")
		return err
	}

	ch := s.channelMembership.GetChannel(channelName)
	if ch == nil {
		return nil
	}

	switch resp.Status {
	case pbChat.SetChannelMuteResponse_Ok:
		// Chatserver broadcasts the AzerothCore-shaped mode-change notification with the target GUID.
	case pbChat.SetChannelMuteResponse_NotMember:
		s.ChannelNotify(ch).Simple(ChatNotMemberNotice)
	case pbChat.SetChannelMuteResponse_NotModerator:
		s.ChannelNotify(ch).Simple(ChatNotModeratorNotice)
	case pbChat.SetChannelMuteResponse_PlayerNotFound:
		s.ChannelNotify(ch).PlayerName(ChatPlayerNotFoundNotice, targetName)
	}

	return nil
}

// HandleChannelUnmute handles CMSG_CHANNEL_UNMUTE - unmute a player
func (s *GameSession) HandleChannelUnmute(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	channelName := r.String()
	if s.forwardNativeChannelPacket(channelName, p) {
		return nil
	}
	channelRealmID, channelTeamID, servicePlayerGUID := s.channelScopeForName(channelName)
	targetName := r.String()

	resp, err := s.chatServiceClient.UnsetChannelMute(ctx, &pbChat.UnsetChannelMuteRequest{
		Api:         root.Ver,
		RealmID:     channelRealmID,
		UnmuterGUID: servicePlayerGUID,
		ChannelName: channelName,
		TargetName:  targetName,
		TeamID:      channelTeamID,
	})
	if err != nil {
		s.logger.Error().Err(err).Str("channelName", channelName).Str("targetName", targetName).Msg("Failed to unmute player")
		return err
	}

	ch := s.channelMembership.GetChannel(channelName)
	if ch == nil {
		return nil
	}

	switch resp.Status {
	case pbChat.UnsetChannelMuteResponse_Ok:
		// Chatserver broadcasts the AzerothCore-shaped mode-change notification with the target GUID.
	case pbChat.UnsetChannelMuteResponse_NotMember:
		s.ChannelNotify(ch).Simple(ChatNotMemberNotice)
	case pbChat.UnsetChannelMuteResponse_NotModerator:
		s.ChannelNotify(ch).Simple(ChatNotModeratorNotice)
	case pbChat.UnsetChannelMuteResponse_PlayerNotFound:
		s.ChannelNotify(ch).PlayerName(ChatPlayerNotFoundNotice, targetName)
	}

	return nil
}

// HandleChannelInvite handles CMSG_CHANNEL_INVITE - invite a player to channel
func (s *GameSession) HandleChannelInvite(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	channelName := r.String()
	if s.forwardNativeChannelPacket(channelName, p) {
		return nil
	}
	channelRealmID, channelTeamID, servicePlayerGUID := s.channelScopeForName(channelName)
	targetName := r.String()

	resp, err := s.chatServiceClient.InviteToChannel(ctx, &pbChat.InviteToChannelRequest{
		Api:         root.Ver,
		RealmID:     channelRealmID,
		InviterGUID: servicePlayerGUID,
		ChannelName: channelName,
		TargetName:  targetName,
		TeamID:      channelTeamID,
	})
	if err != nil {
		s.logger.Error().Err(err).Str("channelName", channelName).Str("targetName", targetName).Msg("Failed to invite to channel")
		return err
	}

	ch := s.channelMembership.GetChannel(channelName)
	if ch == nil {
		return nil
	}

	switch resp.Status {
	case pbChat.InviteToChannelResponse_Ok:
		// Send invitation notification to inviter
		s.ChannelNotify(ch).PlayerName(ChatPlayerInvitedNotice, targetName)
		// The target player receives the invitation notification via the chat event system
	case pbChat.InviteToChannelResponse_NotMember:
		s.ChannelNotify(ch).Simple(ChatNotMemberNotice)
	case pbChat.InviteToChannelResponse_PlayerNotFound:
		s.ChannelNotify(ch).PlayerName(ChatPlayerNotFoundNotice, targetName)
	case pbChat.InviteToChannelResponse_PlayerAlreadyMember:
		// AzerothCore expects a player GUID here; avoid sending a name-shaped packet.
	case pbChat.InviteToChannelResponse_WrongFaction:
		s.ChannelNotify(ch).Simple(ChatInviteWrongFactionNotice)
	case pbChat.InviteToChannelResponse_PlayerBanned:
		s.ChannelNotify(ch).PlayerName(ChatPlayerInviteBannedNotice, targetName)
	}

	return nil
}

// HandleChannelKick handles CMSG_CHANNEL_KICK - kick a player from channel
func (s *GameSession) HandleChannelKick(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	channelName := r.String()
	if s.forwardNativeChannelPacket(channelName, p) {
		return nil
	}
	channelRealmID, channelTeamID, servicePlayerGUID := s.channelScopeForName(channelName)
	targetName := r.String()

	resp, err := s.chatServiceClient.KickFromChannel(ctx, &pbChat.KickFromChannelRequest{
		Api:         root.Ver,
		RealmID:     channelRealmID,
		KickerGUID:  servicePlayerGUID,
		ChannelName: channelName,
		TargetName:  targetName,
		TeamID:      channelTeamID,
	})
	if err != nil {
		s.logger.Error().Err(err).Str("channelName", channelName).Str("targetName", targetName).Msg("Failed to kick from channel")
		return err
	}

	ch := s.channelMembership.GetChannel(channelName)
	if ch == nil {
		return nil
	}

	switch resp.Status {
	case pbChat.KickFromChannelResponse_Ok:
		// Chatserver broadcasts the AzerothCore-shaped kick notification with both player GUIDs.
	case pbChat.KickFromChannelResponse_NotMember:
		s.ChannelNotify(ch).Simple(ChatNotMemberNotice)
	case pbChat.KickFromChannelResponse_NotModerator:
		s.ChannelNotify(ch).Simple(ChatNotModeratorNotice)
	case pbChat.KickFromChannelResponse_PlayerNotFound:
		s.ChannelNotify(ch).PlayerName(ChatPlayerNotFoundNotice, targetName)
	}

	return nil
}

// HandleChannelBan handles CMSG_CHANNEL_BAN - ban a player from channel
func (s *GameSession) HandleChannelBan(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	channelName := r.String()
	if s.forwardNativeChannelPacket(channelName, p) {
		return nil
	}
	channelRealmID, channelTeamID, servicePlayerGUID := s.channelScopeForName(channelName)
	targetName := r.String()

	resp, err := s.chatServiceClient.BanFromChannel(ctx, &pbChat.BanFromChannelRequest{
		Api:         root.Ver,
		RealmID:     channelRealmID,
		BannerGUID:  servicePlayerGUID,
		ChannelName: channelName,
		TargetName:  targetName,
		TeamID:      channelTeamID,
	})
	if err != nil {
		s.logger.Error().Err(err).Str("channelName", channelName).Str("targetName", targetName).Msg("Failed to ban from channel")
		return err
	}

	ch := s.channelMembership.GetChannel(channelName)
	if ch == nil {
		return nil
	}

	switch resp.Status {
	case pbChat.BanFromChannelResponse_Ok:
		// Chatserver broadcasts the AzerothCore-shaped ban notification with both player GUIDs.
	case pbChat.BanFromChannelResponse_NotMember:
		s.ChannelNotify(ch).Simple(ChatNotMemberNotice)
	case pbChat.BanFromChannelResponse_NotModerator:
		s.ChannelNotify(ch).Simple(ChatNotModeratorNotice)
	case pbChat.BanFromChannelResponse_PlayerNotFound:
		s.ChannelNotify(ch).PlayerName(ChatPlayerNotFoundNotice, targetName)
	}

	return nil
}

// HandleChannelUnban handles CMSG_CHANNEL_UNBAN - unban a player from channel
func (s *GameSession) HandleChannelUnban(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	channelName := r.String()
	if s.forwardNativeChannelPacket(channelName, p) {
		return nil
	}
	channelRealmID, channelTeamID, servicePlayerGUID := s.channelScopeForName(channelName)
	targetName := r.String()

	resp, err := s.chatServiceClient.UnbanFromChannel(ctx, &pbChat.UnbanFromChannelRequest{
		Api:          root.Ver,
		RealmID:      channelRealmID,
		UnbannerGUID: servicePlayerGUID,
		ChannelName:  channelName,
		TargetName:   targetName,
		TeamID:       channelTeamID,
	})
	if err != nil {
		s.logger.Error().Err(err).Str("channelName", channelName).Str("targetName", targetName).Msg("Failed to unban from channel")
		return err
	}

	ch := s.channelMembership.GetChannel(channelName)
	if ch == nil {
		return nil
	}

	switch resp.Status {
	case pbChat.UnbanFromChannelResponse_Ok:
		// Chatserver broadcasts the AzerothCore-shaped unban notification with both player GUIDs.
	case pbChat.UnbanFromChannelResponse_NotMember:
		s.ChannelNotify(ch).Simple(ChatNotMemberNotice)
	case pbChat.UnbanFromChannelResponse_NotModerator:
		s.ChannelNotify(ch).Simple(ChatNotModeratorNotice)
	case pbChat.UnbanFromChannelResponse_PlayerNotBanned:
		s.ChannelNotify(ch).PlayerName(ChatPlayerNotBannedNotice, targetName)
	}

	return nil
}

// HandleChannelAnnouncements handles CMSG_CHANNEL_ANNOUNCEMENTS - toggle announcements
func (s *GameSession) HandleChannelAnnouncements(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	channelName := r.String()
	if s.forwardNativeChannelPacket(channelName, p) {
		return nil
	}
	channelRealmID, channelTeamID, servicePlayerGUID := s.channelScopeForName(channelName)

	resp, err := s.chatServiceClient.ToggleChannelAnnouncements(ctx, &pbChat.ToggleChannelAnnouncementsRequest{
		Api:         root.Ver,
		RealmID:     channelRealmID,
		TogglerGUID: servicePlayerGUID,
		ChannelName: channelName,
		TeamID:      channelTeamID,
	})
	if err != nil {
		s.logger.Error().Err(err).Str("channelName", channelName).Msg("Failed to toggle announcements")
		return err
	}

	ch := s.channelMembership.GetChannel(channelName)
	if ch == nil {
		return nil
	}

	switch resp.Status {
	case pbChat.ToggleChannelAnnouncementsResponse_Ok:
		if resp.AnnouncementsEnabled {
			s.ChannelNotify(ch).Simple(ChatAnnouncementsOnNotice)
		} else {
			s.ChannelNotify(ch).Simple(ChatAnnouncementsOffNotice)
		}
	case pbChat.ToggleChannelAnnouncementsResponse_NotMember:
		s.ChannelNotify(ch).Simple(ChatNotMemberNotice)
	case pbChat.ToggleChannelAnnouncementsResponse_NotModerator:
		s.ChannelNotify(ch).Simple(ChatNotModeratorNotice)
	}

	return nil
}

// HandleChannelModerate handles CMSG_CHANNEL_MODERATE - toggle moderation mode
func (s *GameSession) HandleChannelModerate(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	channelName := r.String()
	if s.forwardNativeChannelPacket(channelName, p) {
		return nil
	}
	channelRealmID, channelTeamID, servicePlayerGUID := s.channelScopeForName(channelName)

	resp, err := s.chatServiceClient.ToggleChannelModeration(ctx, &pbChat.ToggleChannelModerationRequest{
		Api:         root.Ver,
		RealmID:     channelRealmID,
		TogglerGUID: servicePlayerGUID,
		ChannelName: channelName,
		TeamID:      channelTeamID,
	})
	if err != nil {
		s.logger.Error().Err(err).Str("channelName", channelName).Msg("Failed to toggle moderation")
		return err
	}

	ch := s.channelMembership.GetChannel(channelName)
	if ch == nil {
		return nil
	}

	switch resp.Status {
	case pbChat.ToggleChannelModerationResponse_Ok:
		if resp.ModerationEnabled {
			s.ChannelNotify(ch).Simple(ChatModerationOnNotice)
		} else {
			s.ChannelNotify(ch).Simple(ChatModerationOffNotice)
		}
	case pbChat.ToggleChannelModerationResponse_NotMember:
		s.ChannelNotify(ch).Simple(ChatNotMemberNotice)
	case pbChat.ToggleChannelModerationResponse_NotModerator:
		s.ChannelNotify(ch).Simple(ChatNotModeratorNotice)
	}

	return nil
}
