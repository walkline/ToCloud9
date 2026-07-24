package session

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

  "github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	root "github.com/walkline/ToCloud9/apps/gateway"
	eBroadcaster "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	pbGuild "github.com/walkline/ToCloud9/gen/guilds/pb"
	"github.com/walkline/ToCloud9/shared/events"
)

type GuildEventType uint8

const (
	GuildEventTypePromotion GuildEventType = iota
	GuildEventTypeDemotion
	GuildEventTypeMessageOfTheDay
	GuildEventTypeJoined
	GuildEventTypeLeft
	GuildEventTypeRemoved
	GuildEventTypeLeaderIS // don't know what is it
	GuildEventTypeLeaderChanged
	GuildEventTypeDisbanded
	GuildEventTypeTabardChanged
	GuildEventTypeRankUpdated
	GuildEventTypeRankDeleted
	GuildEventTypeSignedOn
	GuildEventTypeSignedOff
	GuildEventTypeBankSlotsChanged
	GuildEventTypeTabPurchased
	GuildEventTypeTabUpdated
	GuildEventTypeBankMoneySet
	GuildEventTypeTabAndMoneyUpdated
	GuildEventTypeBankTextChanged
)

func (s *GameSession) HandleGuildRoster(ctx context.Context, p *packet.Packet) error {
	if s.character.GuildID == 0 {
		// TODO: send proper message to the client
		return nil
	}

	guildResp, err := s.guildServiceClient.GetRosterInfo(ctx, &pbGuild.GetRosterInfoParams{
		Api:     root.Ver,
		RealmID: root.RealmID,
		GuildID: uint64(s.character.GuildID),
	})
	if err != nil {
		return err
	}

	resp := packet.NewWriterWithSize(packet.SMsgGuildRoster, 0)
	resp.Uint32(uint32(len(guildResp.Guild.Members)))
	resp.String(guildResp.Guild.WelcomeText)
	resp.String(guildResp.Guild.InfoText)
	resp.Uint32(uint32(len(guildResp.Guild.Ranks)))

	for _, rank := range guildResp.Guild.Ranks {
		resp.Uint32(rank.Flags)
		resp.Uint32(rank.GoldLimit)

		for i := 0; i < 6; i++ {
			if i < len(rank.BankTabRights) {
				resp.Uint32(rank.BankTabRights[i])
			} else {
				resp.Uint32(0)
			}
			if i < len(rank.BankTabSlotsPerDay) {
				resp.Uint32(rank.BankTabSlotsPerDay[i])
			} else {
				resp.Uint32(0)
			}
		}
	}

	for _, member := range guildResp.Guild.Members {
		resp.Uint64(member.Guid)
		resp.Uint8(uint8(member.Status))
		resp.String(member.Name)
		resp.Int32(int32(member.RankID))
		resp.Uint8(uint8(member.Lvl))
		resp.Uint8(uint8(member.ClassID))
		resp.Uint8(uint8(member.Gender))
		resp.Int32(int32(member.AreaID))

		if member.Status == 0 {
			const daySeconds float32 = 60 * 60 * 24
			resp.Float32(float32(time.Now().Unix()-member.LogoutTime) / daySeconds)
		}

		resp.String(member.Note)
		resp.String(member.OfficerNote)
	}

	s.gameSocket.Send(resp)

	return nil
}

func (s *GameSession) GuildLoginCommand(ctx context.Context) error {
	if s.character == nil {
		return nil
	}
	if s.character.GuildID == 0 {
		// TODO: send proper message to the client
		return nil
	}

	guildResp, err := s.guildServiceClient.GetRosterInfo(ctx, &pbGuild.GetRosterInfoParams{
		Api:     root.Ver,
		RealmID: root.RealmID,
		GuildID: uint64(s.character.GuildID),
	})
	if err != nil {
		return err
	}

	s.gameSocket.Send(buildGuildEventPacket(
		GuildEventTypeMessageOfTheDay, 0,
		guildResp.Guild.WelcomeText,
	))

	err = s.HandleGuildRoster(ctx, nil)
	if err != nil {
		return err
	}

	s.gameSocket.Send(buildGuildEventPacket(
		GuildEventTypeSignedOn, s.character.GUID,
		s.character.Name,
	))
	return nil
}

// guildMemberGUIDByName resolves a member of the current character's guild by
// name from the guild roster. Unlike the online characters lookup, it also
// resolves offline members: guild management commands (promote, demote, kick,
// notes) work on offline members too.
func (s *GameSession) guildMemberGUIDByName(ctx context.Context, name string) (uint64, error) {
	if s.character.GuildID == 0 {
		return 0, nil
	}

	guildResp, err := s.guildServiceClient.GetRosterInfo(ctx, &pbGuild.GetRosterInfoParams{
		Api:     root.Ver,
		RealmID: root.RealmID,
		GuildID: uint64(s.character.GuildID),
	})
	if err != nil {
		return 0, err
	}

	for _, member := range guildResp.Guild.Members {
		if strings.EqualFold(member.Name, name) {
			return member.Guid, nil
		}
	}

	return 0, nil
}

// Guild command and result codes carried by SMSG_GUILD_COMMAND_RESULT. The
// client localizes the displayed message from these codes, so no server-side
// translation is needed.
const (
	guildCommandCreate       = 0  // GUILD_COMMAND_CREATE
	guildCommandInvite       = 1  // GUILD_COMMAND_INVITE
	guildErrCommandSuccess   = 0  // ERR_GUILD_COMMAND_SUCCESS
	guildErrAlreadyInGuildS  = 3  // ERR_ALREADY_IN_GUILD_S
	guildErrNameInvalid      = 6  // ERR_GUILD_NAME_INVALID
	guildErrNameExistsS      = 7  // ERR_GUILD_NAME_EXISTS_S
	guildErrPermissions      = 8  // ERR_GUILD_PERMISSIONS
	guildErrPlayerNotInGuild = 9  // ERR_GUILD_PLAYER_NOT_IN_GUILD
	guildErrPlayerNotFoundS  = 11 // ERR_GUILD_PLAYER_NOT_FOUND_S
)

// sendGuildCommandResult sends SMSG_GUILD_COMMAND_RESULT so the client renders
// the localized guild feedback (e.g. "You have invited X into your guild").
func (s *GameSession) sendGuildCommandResult(command uint32, param string, result uint32) {
	w := packet.NewWriterWithSize(packet.SMsgGuildCommandResult, 0)
	w.Uint32(command)
	w.String(param)
	w.Uint32(result)
	s.gameSocket.Send(w)
}

func (s *GameSession) HandleGuildInvite(ctx context.Context, p *packet.Packet) error {
	resp, err := s.charServiceClient.CharacterOnlineByName(ctx, &pbChar.CharacterOnlineByNameRequest{
		Api:           root.Ver,
		RealmID:       root.RealmID,
		CharacterName: p.Reader().String(),
	})
	if err != nil {
		return err
	}

	if resp.Character == nil {
		s.SendSysMessage("Player not found")
		return nil
	}

	// TODO: check fraction.

	_, err = s.guildServiceClient.InviteMember(ctx, &pbGuild.InviteMemberParams{
		Api:         root.Ver,
		RealmID:     root.RealmID,
		Inviter:     s.character.GUID,
		Invitee:     resp.Character.CharGUID,
		InviteeName: resp.Character.CharName,
	})
	if err != nil {
		// Business failures come back as gRPC status codes (see guildserver
		// server layer); surface them to the client instead of a silent log.
		switch status.Code(err) {
		case codes.FailedPrecondition:
			s.sendGuildCommandResult(guildCommandInvite, resp.Character.CharName, guildErrAlreadyInGuildS)
			return nil
		case codes.PermissionDenied:
			s.sendGuildCommandResult(guildCommandInvite, "", guildErrPermissions)
			return nil
		case codes.NotFound:
			s.sendGuildCommandResult(guildCommandInvite, "", guildErrPlayerNotInGuild)
			return nil
		}
		return fmt.Errorf("can't invite member to guild: %w", err)
	}

	s.sendGuildCommandResult(guildCommandInvite, resp.Character.CharName, guildErrCommandSuccess)

	return nil
}

func (s *GameSession) HandleEventGuildInviteCreated(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*eBroadcaster.GuildInviteCreatedPayload)

	resp := packet.NewWriterWithSize(packet.SMsgGuildInvite, 0)
	resp.String(eventData.InviterName)
	resp.String(eventData.GuildName)
	s.gameSocket.Send(resp)

	return nil
}

func (s *GameSession) HandleEventGuildMemberPromoted(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GuildEventMemberPromotePayload)

	s.gameSocket.Send(buildGuildEventPacket(
		GuildEventTypePromotion, 0,
		eventData.PromoterName,
		eventData.MemberName,
		eventData.RankName,
	))

	if eventData.MemberGUID == s.character.GUID {
		// Best effort: a failed push must not abort the remaining event handlers.
		if err := s.sendGuildPermissions(ctx); err != nil {
			log.Warn().Err(err).Msg("can't push guild permissions after rank change")
		}
	}

	return nil
}

func (s *GameSession) HandleEventGuildMemberDemoted(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GuildEventMemberDemotePayload)

	s.gameSocket.Send(buildGuildEventPacket(
		GuildEventTypeDemotion, 0,
		eventData.DemoterName,
		eventData.MemberName,
		eventData.RankName,
	))

	if eventData.MemberGUID == s.character.GUID {
		// Best effort: a failed push must not abort the remaining event handlers.
		if err := s.sendGuildPermissions(ctx); err != nil {
			log.Warn().Err(err).Msg("can't push guild permissions after rank change")
		}
	}

	return nil
}

func (s *GameSession) HandleEventGuildMOTDUpdated(_ context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GuildEventMOTDUpdatedPayload)

	s.gameSocket.Send(buildGuildEventPacket(
		GuildEventTypeMessageOfTheDay, 0,
		eventData.NewMessageOfTheDay,
	))

	return nil
}

func (s *GameSession) HandleEventGuildMemberAdded(_ context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GuildEventMemberAddedPayload)

	s.gameSocket.Send(buildGuildEventPacket(
		GuildEventTypeJoined,
		eventData.MemberGUID,
		eventData.MemberName,
	))

	return nil
}

func (s *GameSession) HandleEventGuildMemberLeft(_ context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GuildEventMemberLeftPayload)

	s.gameSocket.Send(buildGuildEventPacket(
		GuildEventTypeLeft,
		eventData.MemberGUID,
		eventData.MemberName,
	))

	return nil
}

func (s *GameSession) HandleEventGuildMemberKicked(_ context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GuildEventMemberKickedPayload)

	s.gameSocket.Send(buildGuildEventPacket(
		GuildEventTypeRemoved, 0,
		eventData.MemberName,
		eventData.KickerName,
	))

	if s.character.GUID == eventData.MemberGUID {
		s.character.GuildID = 0
	}

	return nil
}

func (s *GameSession) HandleEventGuildRankCreated(_ context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GuildEventRankCreatedPayload)

	s.gameSocket.Send(buildGuildEventPacket(
		GuildEventTypeRankUpdated, 0,
		strconv.Itoa(int(eventData.RankID)),
		eventData.RankName,
		strconv.Itoa(int(eventData.RanksCount)),
	))

	return nil
}

func (s *GameSession) HandleEventGuildRankUpdated(_ context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GuildEventRankUpdatedPayload)

	s.gameSocket.Send(buildGuildEventPacket(
		GuildEventTypeRankUpdated, 0,
		strconv.Itoa(int(eventData.RankID)),
		eventData.RankName,
		strconv.Itoa(int(eventData.RanksCount)),
	))

	return nil
}

func (s *GameSession) HandleEventGuildRankDeleted(_ context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GuildEventRankDeletedPayload)

	s.gameSocket.Send(buildGuildEventPacket(
		GuildEventTypeRankDeleted, 0,
		strconv.Itoa(int(eventData.RanksCount)),
	))

	return nil
}

func (s *GameSession) HandleEventGuildNewMessage(_ context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GuildEventNewMessagePayload)

	resp := packet.NewWriterWithSize(packet.SMsgMessageChat, 0)
	resp.Uint8(uint8(ChatTypeGuild))
	resp.Uint32(eventData.Language)
	resp.Uint64(eventData.SenderGUID)
	resp.Uint32(0) // some flags
	resp.Uint64(eventData.SenderGUID)
	resp.Uint32(uint32(len(eventData.Msg) + 1))
	resp.String(eventData.Msg)
	resp.Uint8(0) // chat tag
	s.gameSocket.Send(resp)

	return nil
}

func (s *GameSession) HandleGuildInviteAccept(ctx context.Context, _ *packet.Packet) error {
	inviteResp, err := s.guildServiceClient.InviteAccepted(ctx, &pbGuild.InviteAcceptedParams{
		Api:     root.Ver,
		RealmID: root.RealmID,
		Character: &pbGuild.InviteAcceptedParams_Character{
			Guid:      s.character.GUID,
			Name:      s.character.Name,
			Lvl:       uint32(s.character.Level),
			Race:      uint32(s.character.Race),
			ClassID:   uint32(s.character.Class),
			Gender:    uint32(s.character.Gender),
			AreaID:    s.character.Zone,
			AccountID: uint64(s.character.AccountID),
		},
	})
	if err != nil {
		return fmt.Errorf("can't accept invite err: %w", err)
	}

	s.character.GuildID = uint32(inviteResp.GuildID)

	return nil
}

func (s *GameSession) HandleGuildLeave(ctx context.Context, p *packet.Packet) error {
	_, err := s.guildServiceClient.Leave(ctx, &pbGuild.LeaveParams{
		Api:     root.Ver,
		RealmID: root.RealmID,
		Leaver:  s.character.GUID,
	})
	if err != nil {
		return fmt.Errorf("can't leave the guild, err: %w", err)
	}

	s.character.GuildID = 0

	return nil
}

func (s *GameSession) HandleGuildKick(ctx context.Context, p *packet.Packet) error {
	targetGUID, err := s.guildMemberGUIDByName(ctx, p.Reader().String())
	if err != nil {
		return err
	}

	if targetGUID == 0 {
		s.SendSysMessage("Player not found")
		return nil
	}

	_, err = s.guildServiceClient.Kick(ctx, &pbGuild.KickParams{
		Api:     root.Ver,
		RealmID: root.RealmID,
		Kicker:  s.character.GUID,
		Target:  targetGUID,
	})
	if err != nil {
		return fmt.Errorf("can't kick player from the guild, err: %w", err)
	}

	return nil
}

func (s *GameSession) HandleGuildSetMessageOfTheDay(ctx context.Context, p *packet.Packet) error {
	_, err := s.guildServiceClient.SetMessageOfTheDay(ctx, &pbGuild.SetMessageOfTheDayParams{
		Api:             root.Ver,
		RealmID:         root.RealmID,
		ChangerGUID:     s.character.GUID,
		MessageOfTheDay: p.Reader().String(),
	})
	if err != nil {
		return fmt.Errorf("can't set message of the day in guild, err: %w", err)
	}

	return nil
}

func (s *GameSession) HandleGuildSetPublicNote(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	targetName := reader.String()
	note := reader.String()

	targetGUID, err := s.guildMemberGUIDByName(ctx, targetName)
	if err != nil {
		return err
	}

	if targetGUID == 0 {
		s.SendSysMessage("Player not found")
		return nil
	}

	_, err = s.guildServiceClient.SetMemberPublicNote(ctx, &pbGuild.SetNoteParams{
		Api:         root.Ver,
		RealmID:     root.RealmID,
		ChangerGUID: s.character.GUID,
		TargetGUID:  targetGUID,
		Note:        note,
	})
	if err != nil {
		return err
	}

	return s.HandleGuildRoster(ctx, p)
}

func (s *GameSession) HandleGuildSetOfficerNote(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	targetName := reader.String()
	note := reader.String()

	targetGUID, err := s.guildMemberGUIDByName(ctx, targetName)
	if err != nil {
		return err
	}

	if targetGUID == 0 {
		s.SendSysMessage("Player not found")
		return nil
	}

	_, err = s.guildServiceClient.SetMemberOfficerNote(ctx, &pbGuild.SetNoteParams{
		Api:         root.Ver,
		RealmID:     root.RealmID,
		ChangerGUID: s.character.GUID,
		TargetGUID:  targetGUID,
		Note:        note,
	})
	if err != nil {
		return err
	}

	return s.HandleGuildRoster(ctx, p)
}

func (s *GameSession) HandleGuildSetInfoText(ctx context.Context, p *packet.Packet) error {
	_, err := s.guildServiceClient.SetGuildInfo(ctx, &pbGuild.SetGuildInfoParams{
		Api:         root.Ver,
		RealmID:     root.RealmID,
		ChangerGUID: s.character.GUID,
		Info:        p.Reader().String(),
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *GameSession) HandleGuildRankUpdate(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	rankID := reader.Uint32()
	rights := reader.Uint32()
	name := reader.String()
	withdrawGoldLimit := reader.Uint32()

	// CMSG_GUILD_RANK carries the bank rights of all six tabs.
	bankTabRights := make([]uint32, 0, 6)
	bankTabSlots := make([]uint32, 0, 6)
	for i := 0; i < 6; i++ {
		bankTabRights = append(bankTabRights, reader.Uint32())
		bankTabSlots = append(bankTabSlots, reader.Uint32())
	}

	_, err := s.guildServiceClient.UpdateRank(ctx, &pbGuild.RankUpdateParams{
		Api:                root.Ver,
		RealmID:            root.RealmID,
		ChangerGUID:        s.character.GUID,
		Rank:               rankID,
		RankName:           name,
		Rights:             rights,
		MoneyPerDay:        withdrawGoldLimit,
		BankTabRights:      bankTabRights,
		BankTabSlotsPerDay: bankTabSlots,
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *GameSession) HandleGuildRankAdd(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	name := reader.String()

	_, err := s.guildServiceClient.AddRank(ctx, &pbGuild.AddRankParams{
		Api:         root.Ver,
		RealmID:     root.RealmID,
		ChangerGUID: s.character.GUID,
		RankName:    name,
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *GameSession) HandleGuildRankDelete(ctx context.Context, p *packet.Packet) error {
	_, err := s.guildServiceClient.DeleteLastRank(ctx, &pbGuild.DeleteLastRankParams{
		Api:         root.Ver,
		RealmID:     root.RealmID,
		ChangerGUID: s.character.GUID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *GameSession) HandleGuildPromote(ctx context.Context, p *packet.Packet) error {
	targetGUID, err := s.guildMemberGUIDByName(ctx, p.Reader().String())
	if err != nil {
		return err
	}

	if targetGUID == 0 {
		s.SendSysMessage("Player not found")
		return nil
	}

	_, err = s.guildServiceClient.PromoteMember(ctx, &pbGuild.PromoteDemoteParams{
		Api:         root.Ver,
		RealmID:     root.RealmID,
		ChangerGUID: s.character.GUID,
		TargetGUID:  targetGUID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *GameSession) HandleGuildDemote(ctx context.Context, p *packet.Packet) error {
	targetGUID, err := s.guildMemberGUIDByName(ctx, p.Reader().String())
	if err != nil {
		return err
	}

	if targetGUID == 0 {
		s.SendSysMessage("Player not found")
		return nil
	}

	_, err = s.guildServiceClient.DemoteMember(ctx, &pbGuild.PromoteDemoteParams{
		Api:         root.Ver,
		RealmID:     root.RealmID,
		ChangerGUID: s.character.GUID,
		TargetGUID:  targetGUID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *GameSession) HandleGuildQuery(ctx context.Context, p *packet.Packet) error {
	guildID := p.Reader().Uint32()
	guildResp, err := s.guildServiceClient.GetGuildInfo(ctx, &pbGuild.GetInfoParams{
		Api:     root.Ver,
		RealmID: root.RealmID,
		GuildID: uint64(guildID),
	})
	if err != nil {
		return err
	}

	resp := packet.NewWriterWithSize(packet.SMsgGuildQueryResponse, 0)
	resp.Uint32(guildID)
	resp.String(guildResp.GuildName)
	for _, name := range guildResp.RankNames {
		resp.String(name)
	}

	resp.Uint32(guildResp.EmblemStyle)
	resp.Uint32(guildResp.EmblemColor)
	resp.Uint32(guildResp.BorderStyle)
	resp.Uint32(guildResp.BorderColor)
	resp.Uint32(guildResp.BackgroundColor)
	resp.Uint32(uint32(len(guildResp.RankNames)))

	s.gameSocket.Send(resp)
	return nil
}

func (s *GameSession) HandleGuildPermissions(ctx context.Context, p *packet.Packet) error {
	return s.sendGuildPermissions(ctx)
}

// sendGuildPermissions pushes the MSG_GUILD_PERMISSIONS response with the
// member's current rank rights. Besides answering the client query, it is
// pushed unsolicited after a promotion/demotion since the client doesn't
// re-query permissions on its own (same trick as the core SendPermissions
// call on guild bank tab purchase).
func (s *GameSession) sendGuildPermissions(ctx context.Context) error {
	if s.character.GuildID == 0 {
		// TODO: send proper message to the client
		return nil
	}

	state, err := s.guildBankState(ctx)
	if err != nil {
		return err
	}

	resp := packet.NewWriterWithSize(packet.MsgGuildPermissions, 0)
	resp.Uint32(state.RankID)
	resp.Int32(int32(state.RankRights))
	resp.Int32(int32(state.MoneyPerDay))
	resp.Uint8(uint8(len(state.Tabs)))

	for i := 0; i < 6; i++ {
		if i < len(state.Tabs) {
			resp.Int32(int32(state.Tabs[i].Rights))
			resp.Int32(int32(state.Tabs[i].RemainingSlots))
		} else {
			resp.Int32(0)
			resp.Int32(0)
		}
	}

	s.gameSocket.Send(resp)
	return nil
}

func (s *GameSession) HandleGuildBankMoneyWithdrawn(ctx context.Context, p *packet.Packet) error {
	return s.sendGuildBankMoneyInfo(ctx)
}

func buildGuildEventPacket(t GuildEventType, guid uint64, args ...string) *packet.Writer {
	resp := packet.NewWriterWithSize(packet.SMsgGuildEvent, 0)
	resp.Uint8(uint8(t))
	resp.Uint8(uint8(len(args)))
	for _, arg := range args {
		resp.String(arg)
	}
	if guid > 0 {
		resp.Uint64(guid)
	}

	return resp
}
