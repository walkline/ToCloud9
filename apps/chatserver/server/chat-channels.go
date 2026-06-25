package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/chatserver"
	"github.com/walkline/ToCloud9/apps/chatserver/service"
	"github.com/walkline/ToCloud9/gen/chat/pb"
	"github.com/walkline/ToCloud9/shared/events"
)

func (s *ChatService) JoinChannel(ctx context.Context, req *pb.JoinChannelRequest) (*pb.JoinChannelResponse, error) {
	channelFlags := uint8(req.ChannelFlags)
	if req.ChannelID == 0 && channelFlags == 0 {
		channelFlags = service.ChannelFlagCustom
	}

	// Get or create the channel with worldserver's flags if provided
	channel, err := s.channelMgr.GetOrCreateChannel(ctx, req.RealmID, req.ChannelName, req.ChannelID, req.TeamID, req.Password, channelFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to get/create channel: %w", err)
	}

	// Try to join
	if err := channel.JoinChannel(ctx, s.channelMgr, req.RealmID, req.PlayerGUID, req.PlayerName, req.Password); err != nil {
		if errors.Is(err, service.ErrPlayerBanned) {
			return &pb.JoinChannelResponse{
				Api:    chatserver.Ver,
				Status: pb.JoinChannelResponse_Banned,
			}, nil
		}
		if errors.Is(err, service.ErrWrongPassword) {
			return &pb.JoinChannelResponse{
				Api:    chatserver.Ver,
				Status: pb.JoinChannelResponse_WrongPassword,
			}, nil
		}
		return nil, err
	}

	// Broadcast join event
	if err := s.broadcastChannelJoined(req.RealmID, channel, req.PlayerGUID, req.PlayerName); err != nil {
		log.Error().Err(err).Msg("Failed to broadcast channel join")
	}

	return &pb.JoinChannelResponse{
		Api:    chatserver.Ver,
		Status: pb.JoinChannelResponse_Ok,
		Channel: &pb.ChannelInfo{
			Name:        channel.GetName(),
			ChannelID:   channel.GetChannelID(),
			Flags:       uint32(channel.GetFlags()),
			NumMembers:  uint32(channel.GetNumMembers()),
			MemberFlags: uint32(channel.GetMemberFlags(req.PlayerGUID)),
		},
	}, nil
}

func (s *ChatService) LeaveChannel(ctx context.Context, req *pb.LeaveChannelRequest) (*pb.LeaveChannelResponse, error) {
	log.Debug().
		Uint32("realmID", req.RealmID).
		Uint64("playerGUID", req.PlayerGUID).
		Str("channelName", req.ChannelName).
		Msg("LeaveChannel request")

	playerName, isCustom, newOwnerGUID, oldFlags, newFlags, err := s.channelMgr.LeaveChannelByGUID(ctx, req.RealmID, req.ChannelName, req.TeamID, req.PlayerGUID)
	if err != nil {
		if errors.Is(err, service.ErrNotMember) || errors.Is(err, service.ErrChannelNotFound) {
			return &pb.LeaveChannelResponse{
				Api:    chatserver.Ver,
				Status: pb.LeaveChannelResponse_NotMember,
			}, nil
		}
		return nil, err
	}

	// Broadcast leave event, but only for custom channels
	if isCustom {
		channel := s.channelMgr.GetChannel(req.RealmID, req.ChannelName, req.TeamID)
		if channel != nil {
			if err := s.broadcastChannelLeft(req.RealmID, channel, req.PlayerGUID, playerName, false); err != nil {
				log.Error().Err(err).Msg("Failed to broadcast channel leave")
			}

			// If ownership was transferred, broadcast the ownership change
			if newOwnerGUID != 0 {
				if err := s.broadcastModeChange(req.RealmID, channel, newOwnerGUID, oldFlags, newFlags); err != nil {
					log.Error().Err(err).Msg("Failed to broadcast owner mode change after leave")
				}
				if err := s.broadcastOwnerChanged(req.RealmID, channel, newOwnerGUID); err != nil {
					log.Error().Err(err).Msg("Failed to broadcast owner changed after leave")
				}
			}
		}
	}

	return &pb.LeaveChannelResponse{
		Api:    chatserver.Ver,
		Status: pb.LeaveChannelResponse_Ok,
	}, nil
}

func (s *ChatService) SendChannelMessage(ctx context.Context, req *pb.SendChannelMessageRequest) (*pb.SendChannelMessageResponse, error) {
	channel, err := s.channelMgr.ValidateSendMessage(req.RealmID, req.ChannelName, req.TeamID, req.SenderGUID)
	if err != nil {
		if errors.Is(err, service.ErrNotMember) {
			return &pb.SendChannelMessageResponse{
				Api:    chatserver.Ver,
				Status: pb.SendChannelMessageResponse_NotMember,
			}, nil
		}
		if errors.Is(err, service.ErrMuted) {
			return &pb.SendChannelMessageResponse{
				Api:    chatserver.Ver,
				Status: pb.SendChannelMessageResponse_Muted,
			}, nil
		}
		return nil, err
	}

	// Broadcast the message
	if err := s.broadcastChannelMessage(req.RealmID, channel, req.SenderGUID, req.SenderName, req.Language, req.Message, uint8(req.SenderChatTag)); err != nil {
		log.Error().Err(err).Msg("Failed to broadcast channel message")
		return nil, err
	}

	// Update last used timestamp (fire and forget)
	go func() {
		if err := s.channelMgr.UpdateLastUsed(context.Background(), req.RealmID, req.ChannelName, req.TeamID); err != nil {
			log.Error().Err(err).Msg("Failed to update channel last used timestamp")
		}
	}()

	return &pb.SendChannelMessageResponse{
		Api:    chatserver.Ver,
		Status: pb.SendChannelMessageResponse_Ok,
	}, nil
}

func (s *ChatService) GetChannelList(ctx context.Context, req *pb.GetChannelListRequest) (*pb.GetChannelListResponse, error) {
	channel := s.channelMgr.GetChannel(req.RealmID, req.ChannelName, req.TeamID)
	if channel == nil {
		return &pb.GetChannelListResponse{
			Api:    chatserver.Ver,
			Status: pb.GetChannelListResponse_NotMember,
		}, nil
	}

	if !channel.IsMember(req.PlayerGUID) {
		return &pb.GetChannelListResponse{
			Api:    chatserver.Ver,
			Status: pb.GetChannelListResponse_NotMember,
		}, nil
	}

	members := channel.GetMembers()
	pbMembers := make([]*pb.ChannelMember, len(members))
	for i, m := range members {
		pbMembers[i] = &pb.ChannelMember{
			Guid:  m.PlayerGUID,
			Name:  m.PlayerName,
			Flags: uint32(m.Flags),
		}
	}

	return &pb.GetChannelListResponse{
		Api:     chatserver.Ver,
		Status:  pb.GetChannelListResponse_Ok,
		Members: pbMembers,
	}, nil
}

func (s *ChatService) KickFromChannel(ctx context.Context, req *pb.KickFromChannelRequest) (*pb.KickFromChannelResponse, error) {
	channel, targetGUID, newOwnerGUID, oldFlags, newFlags, err := s.channelMgr.KickPlayer(ctx, req.RealmID, req.ChannelName, req.TeamID, req.KickerGUID, req.TargetName)
	if err != nil {
		if errors.Is(err, service.ErrNotMember) || errors.Is(err, service.ErrChannelNotFound) {
			return &pb.KickFromChannelResponse{
				Api:    chatserver.Ver,
				Status: pb.KickFromChannelResponse_NotMember,
			}, nil
		}
		if errors.Is(err, service.ErrNotModerator) {
			return &pb.KickFromChannelResponse{
				Api:    chatserver.Ver,
				Status: pb.KickFromChannelResponse_NotModerator,
			}, nil
		}
		if errors.Is(err, service.ErrPlayerNotFound) {
			return &pb.KickFromChannelResponse{
				Api:    chatserver.Ver,
				Status: pb.KickFromChannelResponse_PlayerNotFound,
			}, nil
		}
		return nil, err
	}

	if err := s.broadcastPlayerKicked(req.RealmID, channel, targetGUID, req.KickerGUID); err != nil {
		log.Error().Err(err).Msg("Failed to broadcast channel kick notification")
	}

	if err := s.broadcastChannelLeft(req.RealmID, channel, targetGUID, req.TargetName, true); err != nil {
		log.Error().Err(err).Msg("Failed to broadcast silent channel kick")
	}

	if newOwnerGUID != 0 {
		if err := s.broadcastModeChange(req.RealmID, channel, newOwnerGUID, oldFlags, newFlags); err != nil {
			log.Error().Err(err).Msg("Failed to broadcast owner mode change after kick")
		}
		if err := s.broadcastOwnerChanged(req.RealmID, channel, newOwnerGUID); err != nil {
			log.Error().Err(err).Msg("Failed to broadcast owner changed after kick")
		}
	}

	return &pb.KickFromChannelResponse{
		Api:    chatserver.Ver,
		Status: pb.KickFromChannelResponse_Ok,
	}, nil
}

func (s *ChatService) BanFromChannel(ctx context.Context, req *pb.BanFromChannelRequest) (*pb.BanFromChannelResponse, error) {
	channel, targetGUID, newOwnerGUID, oldFlags, newFlags, err := s.channelMgr.BanPlayerByName(ctx, req.RealmID, req.ChannelName, req.TeamID, req.BannerGUID, req.TargetName)
	if err != nil {
		if errors.Is(err, service.ErrNotMember) || errors.Is(err, service.ErrChannelNotFound) {
			return &pb.BanFromChannelResponse{
				Api:    chatserver.Ver,
				Status: pb.BanFromChannelResponse_NotMember,
			}, nil
		}
		if errors.Is(err, service.ErrNotModerator) {
			return &pb.BanFromChannelResponse{
				Api:    chatserver.Ver,
				Status: pb.BanFromChannelResponse_NotModerator,
			}, nil
		}
		if errors.Is(err, service.ErrPlayerNotFound) {
			return &pb.BanFromChannelResponse{
				Api:    chatserver.Ver,
				Status: pb.BanFromChannelResponse_PlayerNotFound,
			}, nil
		}
		log.Error().Err(err).Msg("Failed to ban player")
		return nil, err
	}

	if err := s.broadcastPlayerBanned(req.RealmID, channel, targetGUID, req.BannerGUID); err != nil {
		log.Error().Err(err).Msg("Failed to broadcast channel ban notification")
	}

	if err := s.broadcastChannelLeft(req.RealmID, channel, targetGUID, req.TargetName, true); err != nil {
		log.Error().Err(err).Msg("Failed to broadcast silent channel ban")
	}

	if newOwnerGUID != 0 {
		if err := s.broadcastModeChange(req.RealmID, channel, newOwnerGUID, oldFlags, newFlags); err != nil {
			log.Error().Err(err).Msg("Failed to broadcast owner mode change after ban")
		}
		if err := s.broadcastOwnerChanged(req.RealmID, channel, newOwnerGUID); err != nil {
			log.Error().Err(err).Msg("Failed to broadcast owner changed after ban")
		}
	}

	return &pb.BanFromChannelResponse{
		Api:    chatserver.Ver,
		Status: pb.BanFromChannelResponse_Ok,
	}, nil
}

func (s *ChatService) UnbanFromChannel(ctx context.Context, req *pb.UnbanFromChannelRequest) (*pb.UnbanFromChannelResponse, error) {
	targetGUID, err := s.channelMgr.UnbanPlayerByName(ctx, req.RealmID, req.ChannelName, req.TeamID, req.UnbannerGUID, req.TargetName)
	if err != nil {
		if errors.Is(err, service.ErrChannelNotFound) {
			return &pb.UnbanFromChannelResponse{
				Api:    chatserver.Ver,
				Status: pb.UnbanFromChannelResponse_NotMember,
			}, nil
		}
		if errors.Is(err, service.ErrNotModerator) {
			return &pb.UnbanFromChannelResponse{
				Api:    chatserver.Ver,
				Status: pb.UnbanFromChannelResponse_NotModerator,
			}, nil
		}
		if errors.Is(err, service.ErrPlayerNotFound) {
			return &pb.UnbanFromChannelResponse{
				Api:    chatserver.Ver,
				Status: pb.UnbanFromChannelResponse_PlayerNotBanned,
			}, nil
		}
		log.Error().Err(err).Msg("Failed to unban player")
		return nil, err
	}

	channel := s.channelMgr.GetChannel(req.RealmID, req.ChannelName, req.TeamID)
	if channel != nil {
		if err := s.broadcastPlayerUnbanned(req.RealmID, channel, targetGUID, req.UnbannerGUID); err != nil {
			log.Error().Err(err).Msg("Failed to broadcast channel unban notification")
		}
	}

	return &pb.UnbanFromChannelResponse{
		Api:    chatserver.Ver,
		Status: pb.UnbanFromChannelResponse_Ok,
	}, nil
}

func (s *ChatService) SetChannelModerator(ctx context.Context, req *pb.SetChannelModeratorRequest) (*pb.SetChannelModeratorResponse, error) {
	targetGUID, oldFlags, newFlags, err := s.channelMgr.SetModeratorByName(ctx, req.RealmID, req.ChannelName, req.TeamID, req.SetterGUID, req.TargetName, true)
	if err != nil {
		if errors.Is(err, service.ErrNotMember) || errors.Is(err, service.ErrChannelNotFound) {
			return &pb.SetChannelModeratorResponse{
				Api:    chatserver.Ver,
				Status: pb.SetChannelModeratorResponse_NotMember,
			}, nil
		}
		if errors.Is(err, service.ErrNotOwner) {
			return &pb.SetChannelModeratorResponse{
				Api:    chatserver.Ver,
				Status: pb.SetChannelModeratorResponse_NotOwner,
			}, nil
		}
		if errors.Is(err, service.ErrPlayerNotFound) {
			return &pb.SetChannelModeratorResponse{
				Api:    chatserver.Ver,
				Status: pb.SetChannelModeratorResponse_PlayerNotFound,
			}, nil
		}
		return nil, err
	}

	// Broadcast mode change notification to all channel members
	channel := s.channelMgr.GetChannel(req.RealmID, req.ChannelName, req.TeamID)
	if channel != nil {
		if err := s.broadcastModeChange(req.RealmID, channel, targetGUID, oldFlags, newFlags); err != nil {
			log.Error().Err(err).Msg("Failed to broadcast moderator change")
		}
	}

	return &pb.SetChannelModeratorResponse{
		Api:    chatserver.Ver,
		Status: pb.SetChannelModeratorResponse_Ok,
	}, nil
}

func (s *ChatService) UnsetChannelModerator(ctx context.Context, req *pb.UnsetChannelModeratorRequest) (*pb.UnsetChannelModeratorResponse, error) {
	targetGUID, oldFlags, newFlags, err := s.channelMgr.SetModeratorByName(ctx, req.RealmID, req.ChannelName, req.TeamID, req.SetterGUID, req.TargetName, false)
	if err != nil {
		if errors.Is(err, service.ErrNotMember) || errors.Is(err, service.ErrChannelNotFound) {
			return &pb.UnsetChannelModeratorResponse{
				Api:    chatserver.Ver,
				Status: pb.UnsetChannelModeratorResponse_NotMember,
			}, nil
		}
		if errors.Is(err, service.ErrNotOwner) {
			return &pb.UnsetChannelModeratorResponse{
				Api:    chatserver.Ver,
				Status: pb.UnsetChannelModeratorResponse_NotOwner,
			}, nil
		}
		if errors.Is(err, service.ErrPlayerNotFound) {
			return &pb.UnsetChannelModeratorResponse{
				Api:    chatserver.Ver,
				Status: pb.UnsetChannelModeratorResponse_PlayerNotFound,
			}, nil
		}
		return nil, err
	}

	// Broadcast mode change notification to all channel members
	channel := s.channelMgr.GetChannel(req.RealmID, req.ChannelName, req.TeamID)
	if channel != nil {
		if err := s.broadcastModeChange(req.RealmID, channel, targetGUID, oldFlags, newFlags); err != nil {
			log.Error().Err(err).Msg("Failed to broadcast moderator change")
		}
	}

	return &pb.UnsetChannelModeratorResponse{
		Api:    chatserver.Ver,
		Status: pb.UnsetChannelModeratorResponse_Ok,
	}, nil
}

func (s *ChatService) SetChannelMute(ctx context.Context, req *pb.SetChannelMuteRequest) (*pb.SetChannelMuteResponse, error) {
	targetGUID, oldFlags, newFlags, err := s.channelMgr.SetMuteByName(ctx, req.RealmID, req.ChannelName, req.TeamID, req.MuterGUID, req.TargetName, true)
	if err != nil {
		if errors.Is(err, service.ErrNotMember) || errors.Is(err, service.ErrChannelNotFound) {
			return &pb.SetChannelMuteResponse{
				Api:    chatserver.Ver,
				Status: pb.SetChannelMuteResponse_NotMember,
			}, nil
		}
		if errors.Is(err, service.ErrNotModerator) {
			return &pb.SetChannelMuteResponse{
				Api:    chatserver.Ver,
				Status: pb.SetChannelMuteResponse_NotModerator,
			}, nil
		}
		if errors.Is(err, service.ErrPlayerNotFound) {
			return &pb.SetChannelMuteResponse{
				Api:    chatserver.Ver,
				Status: pb.SetChannelMuteResponse_PlayerNotFound,
			}, nil
		}
		return nil, err
	}

	// Broadcast mode change notification to all channel members
	channel := s.channelMgr.GetChannel(req.RealmID, req.ChannelName, req.TeamID)
	if channel != nil {
		if err := s.broadcastModeChange(req.RealmID, channel, targetGUID, oldFlags, newFlags); err != nil {
			log.Error().Err(err).Msg("Failed to broadcast mute change")
		}
	}

	return &pb.SetChannelMuteResponse{
		Api:    chatserver.Ver,
		Status: pb.SetChannelMuteResponse_Ok,
	}, nil
}

func (s *ChatService) UnsetChannelMute(ctx context.Context, req *pb.UnsetChannelMuteRequest) (*pb.UnsetChannelMuteResponse, error) {
	targetGUID, oldFlags, newFlags, err := s.channelMgr.SetMuteByName(ctx, req.RealmID, req.ChannelName, req.TeamID, req.UnmuterGUID, req.TargetName, false)
	if err != nil {
		if errors.Is(err, service.ErrNotMember) || errors.Is(err, service.ErrChannelNotFound) {
			return &pb.UnsetChannelMuteResponse{
				Api:    chatserver.Ver,
				Status: pb.UnsetChannelMuteResponse_NotMember,
			}, nil
		}
		if errors.Is(err, service.ErrNotModerator) {
			return &pb.UnsetChannelMuteResponse{
				Api:    chatserver.Ver,
				Status: pb.UnsetChannelMuteResponse_NotModerator,
			}, nil
		}
		if errors.Is(err, service.ErrPlayerNotFound) {
			return &pb.UnsetChannelMuteResponse{
				Api:    chatserver.Ver,
				Status: pb.UnsetChannelMuteResponse_PlayerNotFound,
			}, nil
		}
		return nil, err
	}

	// Broadcast mode change notification to all channel members
	channel := s.channelMgr.GetChannel(req.RealmID, req.ChannelName, req.TeamID)
	if channel != nil {
		if err := s.broadcastModeChange(req.RealmID, channel, targetGUID, oldFlags, newFlags); err != nil {
			log.Error().Err(err).Msg("Failed to broadcast unmute change")
		}
	}

	return &pb.UnsetChannelMuteResponse{
		Api:    chatserver.Ver,
		Status: pb.UnsetChannelMuteResponse_Ok,
	}, nil
}

func (s *ChatService) SetChannelOwner(ctx context.Context, req *pb.SetChannelOwnerRequest) (*pb.SetChannelOwnerResponse, error) {
	targetGUID, oldFlags, newFlags, err := s.channelMgr.SetOwnerByName(ctx, req.RealmID, req.ChannelName, req.TeamID, req.SetterGUID, req.TargetName)
	if err != nil {
		if errors.Is(err, service.ErrChannelNotFound) {
			return &pb.SetChannelOwnerResponse{
				Api:    chatserver.Ver,
				Status: pb.SetChannelOwnerResponse_NotMember,
			}, nil
		}
		if errors.Is(err, service.ErrNotOwner) {
			return &pb.SetChannelOwnerResponse{
				Api:    chatserver.Ver,
				Status: pb.SetChannelOwnerResponse_NotOwner,
			}, nil
		}
		if errors.Is(err, service.ErrPlayerNotFound) {
			return &pb.SetChannelOwnerResponse{
				Api:    chatserver.Ver,
				Status: pb.SetChannelOwnerResponse_PlayerNotFound,
			}, nil
		}
		log.Error().Err(err).Msg("Failed to transfer channel ownership")
		return nil, err
	}

	// Broadcast mode change and owner changed notifications to all channel members
	channel := s.channelMgr.GetChannel(req.RealmID, req.ChannelName, req.TeamID)
	if channel != nil {
		// First send mode change (same as C++ line 893-894)
		if err := s.broadcastModeChange(req.RealmID, channel, targetGUID, oldFlags, newFlags); err != nil {
			log.Error().Err(err).Msg("Failed to broadcast owner mode change")
		}
		// Then send owner changed notification with GUID (same as C++ line 899-900)
		if err := s.broadcastOwnerChanged(req.RealmID, channel, targetGUID); err != nil {
			log.Error().Err(err).Msg("Failed to broadcast owner changed")
		}
	}

	return &pb.SetChannelOwnerResponse{
		Api:    chatserver.Ver,
		Status: pb.SetChannelOwnerResponse_Ok,
	}, nil
}

func (s *ChatService) SetChannelPassword(ctx context.Context, req *pb.SetChannelPasswordRequest) (*pb.SetChannelPasswordResponse, error) {
	err := s.channelMgr.SetChannelPassword(ctx, req.RealmID, req.ChannelName, req.TeamID, req.SetterGUID, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrChannelNotFound) {
			return &pb.SetChannelPasswordResponse{
				Api:    chatserver.Ver,
				Status: pb.SetChannelPasswordResponse_NotMember,
			}, nil
		}
		if errors.Is(err, service.ErrNotOwner) {
			return &pb.SetChannelPasswordResponse{
				Api:    chatserver.Ver,
				Status: pb.SetChannelPasswordResponse_NotOwner,
			}, nil
		}
		log.Error().Err(err).Msg("Failed to update channel password")
		return nil, err
	}

	return &pb.SetChannelPasswordResponse{
		Api:    chatserver.Ver,
		Status: pb.SetChannelPasswordResponse_Ok,
	}, nil
}

func (s *ChatService) ToggleChannelModeration(ctx context.Context, req *pb.ToggleChannelModerationRequest) (*pb.ToggleChannelModerationResponse, error) {
	enabled, err := s.channelMgr.ToggleChannelModeration(req.RealmID, req.ChannelName, req.TeamID, req.TogglerGUID)
	if err != nil {
		if errors.Is(err, service.ErrNotMember) || errors.Is(err, service.ErrChannelNotFound) {
			return &pb.ToggleChannelModerationResponse{
				Api:    chatserver.Ver,
				Status: pb.ToggleChannelModerationResponse_NotMember,
			}, nil
		}
		if errors.Is(err, service.ErrNotModerator) {
			return &pb.ToggleChannelModerationResponse{
				Api:    chatserver.Ver,
				Status: pb.ToggleChannelModerationResponse_NotModerator,
			}, nil
		}
		return nil, err
	}

	return &pb.ToggleChannelModerationResponse{
		Api:               chatserver.Ver,
		Status:            pb.ToggleChannelModerationResponse_Ok,
		ModerationEnabled: enabled,
	}, nil
}

func (s *ChatService) ToggleChannelAnnouncements(ctx context.Context, req *pb.ToggleChannelAnnouncementsRequest) (*pb.ToggleChannelAnnouncementsResponse, error) {
	enabled, err := s.channelMgr.ToggleChannelAnnouncements(ctx, req.RealmID, req.ChannelName, req.TeamID, req.TogglerGUID)
	if err != nil {
		if errors.Is(err, service.ErrNotMember) || errors.Is(err, service.ErrChannelNotFound) {
			return &pb.ToggleChannelAnnouncementsResponse{
				Api:    chatserver.Ver,
				Status: pb.ToggleChannelAnnouncementsResponse_NotMember,
			}, nil
		}
		if errors.Is(err, service.ErrNotModerator) {
			return &pb.ToggleChannelAnnouncementsResponse{
				Api:    chatserver.Ver,
				Status: pb.ToggleChannelAnnouncementsResponse_NotModerator,
			}, nil
		}
		log.Error().Err(err).Msg("Failed to toggle announcements")
		return nil, err
	}

	return &pb.ToggleChannelAnnouncementsResponse{
		Api:                  chatserver.Ver,
		Status:               pb.ToggleChannelAnnouncementsResponse_Ok,
		AnnouncementsEnabled: enabled,
	}, nil
}

func (s *ChatService) InviteToChannel(ctx context.Context, req *pb.InviteToChannelRequest) (*pb.InviteToChannelResponse, error) {
	targetChar, err := s.charRepo.CharacterByRealmAndName(ctx, req.RealmID, req.TargetName)
	if err != nil || targetChar == nil {
		return &pb.InviteToChannelResponse{
			Api:    chatserver.Ver,
			Status: pb.InviteToChannelResponse_PlayerNotFound,
		}, nil
	}

	// Get channel
	channel := s.channelMgr.GetChannel(req.RealmID, req.ChannelName, req.TeamID)
	if channel == nil {
		return &pb.InviteToChannelResponse{
			Api:    chatserver.Ver,
			Status: pb.InviteToChannelResponse_NotMember,
		}, nil
	}

	// Check inviter is a member
	if !channel.IsMember(req.InviterGUID) {
		return &pb.InviteToChannelResponse{
			Api:    chatserver.Ver,
			Status: pb.InviteToChannelResponse_NotMember,
		}, nil
	}

	// Check target is not already a member
	if channel.IsMember(targetChar.GUID) {
		return &pb.InviteToChannelResponse{
			Api:    chatserver.Ver,
			Status: pb.InviteToChannelResponse_PlayerAlreadyMember,
		}, nil
	}

	// TODO: Check faction match when we have faction info
	// TODO: Check if target is banned

	// Send invitation notification to target via event system
	// ChatInviteNotice = 0x18
	payload := &events.ChatEventChannelNotificationPayload{
		RealmID:       req.RealmID,
		ChannelName:   req.ChannelName,
		ChannelID:     channel.GetChannelID(),
		TeamID:        uint32(channel.GetTeamID()),
		NotifyType:    0x18,            // ChatInviteNotice
		TargetGUID:    req.InviterGUID, // The inviter's GUID (shown in the packet)
		TargetName:    req.TargetName,
		SecondGUID:    req.InviterGUID,
		AffectsPlayer: targetChar.GUID, // Send ONLY to the invited player
	}

	if err := s.msgProducer.ProduceChannelNotification(payload); err != nil {
		log.Error().Err(err).Msg("Failed to send channel invitation event")
	}

	return &pb.InviteToChannelResponse{
		Api:    chatserver.Ver,
		Status: pb.InviteToChannelResponse_Ok,
	}, nil
}

// Broadcast helpers

func (s *ChatService) broadcastChannelMessage(realmID uint32, channel *service.ActiveChannel, senderGUID uint64, senderName string, language uint32, message string, senderChatTag uint8) error {
	payload := &events.ChatEventChannelMessagePayload{
		RealmID:       realmID,
		ChannelName:   channel.GetName(),
		ChannelID:     channel.GetChannelID(),
		TeamID:        uint32(channel.GetTeamID()),
		SenderGUID:    senderGUID,
		SenderName:    senderName,
		Language:      language,
		Message:       message,
		SenderChatTag: senderChatTag,
	}

	return s.msgProducer.ProduceChannelMessage(payload)
}

func (s *ChatService) broadcastChannelJoined(realmID uint32, channel *service.ActiveChannel, playerGUID uint64, playerName string) error {
	payload := &events.ChatEventChannelJoinedPayload{
		ServiceID:    s.serviceID,
		RealmID:      realmID,
		ChannelName:  channel.GetName(),
		ChannelID:    channel.GetChannelID(),
		ChannelFlags: uint32(channel.GetFlags()),
		TeamID:       uint32(channel.GetTeamID()),
		NumMembers:   uint32(channel.GetNumMembers()),
		PlayerGUID:   playerGUID,
		PlayerName:   playerName,
		PlayerFlags:  channel.GetMemberFlags(playerGUID),
	}

	return s.msgProducer.ProduceChannelJoined(payload)
}

func (s *ChatService) broadcastChannelLeft(realmID uint32, channel *service.ActiveChannel, playerGUID uint64, playerName string, silent bool) error {
	payload := &events.ChatEventChannelLeftPayload{
		ServiceID:    s.serviceID,
		RealmID:      realmID,
		ChannelName:  channel.GetName(),
		ChannelID:    channel.GetChannelID(),
		ChannelFlags: uint32(channel.GetFlags()),
		TeamID:       uint32(channel.GetTeamID()),
		NumMembers:   uint32(channel.GetNumMembers()),
		PlayerGUID:   playerGUID,
		PlayerName:   playerName,
		Silent:       silent,
	}

	return s.msgProducer.ProduceChannelLeft(payload)
}

func (s *ChatService) broadcastModeChange(realmID uint32, channel *service.ActiveChannel, targetGUID uint64, oldFlags uint8, newFlags uint8) error {
	payload := &events.ChatEventChannelNotificationPayload{
		RealmID:      realmID,
		ChannelName:  channel.GetName(),
		ChannelID:    channel.GetChannelID(),
		ChannelFlags: uint32(channel.GetFlags()),
		TeamID:       uint32(channel.GetTeamID()),
		NumMembers:   uint32(channel.GetNumMembers()),
		NotifyType:   0x0C, // CHAT_MODE_CHANGE_NOTICE
		TargetGUID:   targetGUID,
		OldFlags:     oldFlags,
		NewFlags:     newFlags,
	}

	return s.msgProducer.ProduceChannelNotification(payload)
}

func (s *ChatService) broadcastOwnerChanged(realmID uint32, channel *service.ActiveChannel, newOwnerGUID uint64) error {
	payload := &events.ChatEventChannelNotificationPayload{
		RealmID:      realmID,
		ChannelName:  channel.GetName(),
		ChannelID:    channel.GetChannelID(),
		ChannelFlags: uint32(channel.GetFlags()),
		TeamID:       uint32(channel.GetTeamID()),
		NumMembers:   uint32(channel.GetNumMembers()),
		NotifyType:   0x08, // CHAT_OWNER_CHANGED_NOTICE
		TargetGUID:   newOwnerGUID,
	}

	return s.msgProducer.ProduceChannelNotification(payload)
}

func (s *ChatService) broadcastPlayerKicked(realmID uint32, channel *service.ActiveChannel, targetGUID, actorGUID uint64) error {
	return s.broadcastTwoPlayerChannelNotification(realmID, channel, 0x12, targetGUID, actorGUID)
}

func (s *ChatService) broadcastPlayerBanned(realmID uint32, channel *service.ActiveChannel, targetGUID, actorGUID uint64) error {
	return s.broadcastTwoPlayerChannelNotification(realmID, channel, 0x14, targetGUID, actorGUID)
}

func (s *ChatService) broadcastPlayerUnbanned(realmID uint32, channel *service.ActiveChannel, targetGUID, actorGUID uint64) error {
	return s.broadcastTwoPlayerChannelNotification(realmID, channel, 0x15, targetGUID, actorGUID)
}

func (s *ChatService) broadcastTwoPlayerChannelNotification(realmID uint32, channel *service.ActiveChannel, notifyType uint8, targetGUID, actorGUID uint64) error {
	payload := &events.ChatEventChannelNotificationPayload{
		RealmID:      realmID,
		ChannelName:  channel.GetName(),
		ChannelID:    channel.GetChannelID(),
		ChannelFlags: uint32(channel.GetFlags()),
		TeamID:       uint32(channel.GetTeamID()),
		NumMembers:   uint32(channel.GetNumMembers()),
		NotifyType:   notifyType,
		TargetGUID:   targetGUID,
		SecondGUID:   actorGUID,
	}

	return s.msgProducer.ProduceChannelNotification(payload)
}
