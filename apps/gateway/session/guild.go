package session

import (
	"context"
	"fmt"
	"strconv"
	"time"

	root "github.com/walkline/ToCloud9/apps/gateway"
	eBroadcaster "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	pbGuild "github.com/walkline/ToCloud9/gen/guilds/pb"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/wow"
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

const (
	guildCommandCreate                      int32  = 0
	guildCommandInvite                      int32  = 1
	guildCommandResultSuccess               int32  = 0
	guildCommandResultAlreadyInGuildS       int32  = 3
	guildCommandResultAlreadyInvitedToGuild int32  = 5
	guildCommandResultPlayerNotFound        int32  = 11
	guildCommandResultNotAllied             int32  = 12
	petitionTurnOK                          uint32 = 0
	petitionTurnNeedMoreSignatures          uint32 = 4
	petitionSignOK                          uint32 = 0
	petitionSignAlreadySigned               uint32 = 1
	petitionSignAlreadyInGuild              uint32 = 2
	petitionSignCantSignOwn                 uint32 = 3
	petitionSignNotServer                   uint32 = 4
	guildPetitionType                       uint32 = 9
	guildBankMaxTabs                               = 6
	guildBankWithdrawMoneyIdx                      = guildBankMaxTabs
	guildWithdrawUnlimited                  uint32 = 0xFFFFFFFF
	guildBankRightViewTab                   uint32 = 0x01
	guildRightWithdrawRepair                uint32 = 0x00040000
	guildRightWithdrawGold                  uint32 = 0x00080000
)

func (s *GameSession) HandleGuildRoster(ctx context.Context, p *packet.Packet) error {
	if s.character.GuildID == 0 {
		// TODO: send proper message to the client
		return nil
	}

	guildResp, err := s.guildServiceClient.GetRosterInfo(ctx, &pbGuild.GetRosterInfoParams{
		Api:     root.Ver,
		RealmID: s.guildHomeRealmID(),
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
		writeGuildBankTabRights(resp, rank)
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
		RealmID: s.guildHomeRealmID(),
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

func (s *GameSession) HandleGuildInvite(ctx context.Context, p *packet.Packet) error {
	targetName := p.Reader().String()
	resp, err := s.charServiceClient.CharacterOnlineByName(ctx, &pbChar.CharacterOnlineByNameRequest{
		Api:           root.Ver,
		RealmID:       s.guildHomeRealmID(),
		CharacterName: targetName,
	})
	if err != nil {
		return err
	}

	if resp.Character == nil {
		s.sendGuildCommandResult(guildCommandInvite, targetName, guildCommandResultPlayerNotFound)
		return nil
	}

	if resp.Character.GetRealmID() != s.guildHomeRealmID() {
		s.sendGuildCommandResult(guildCommandInvite, targetName, guildCommandResultPlayerNotFound)
		return nil
	}

	if !s.guildCanInviteRace(uint8(resp.Character.GetCharRace())) {
		s.sendGuildCommandResult(guildCommandInvite, resp.Character.GetCharName(), guildCommandResultNotAllied)
		return nil
	}

	_, err = s.guildServiceClient.InviteMember(ctx, &pbGuild.InviteMemberParams{
		Api:               root.Ver,
		RealmID:           s.guildHomeRealmID(),
		Inviter:           s.character.GUID,
		Invitee:           resp.Character.CharGUID,
		InviteeName:       resp.Character.CharName,
		InviteeRace:       resp.Character.CharRace,
		AllowCrossFaction: root.AllowTwoSideInteractionGuild,
	})

	return err
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

	if s.character != nil && s.character.GUID == eventData.MemberGUID {
		s.character.GuildID = uint32(eventData.GuildID)
	}

	return nil
}

func (s *GameSession) HandleEventGuildMemberLeft(_ context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GuildEventMemberLeftPayload)

	s.gameSocket.Send(buildGuildEventPacket(
		GuildEventTypeLeft,
		eventData.MemberGUID,
		eventData.MemberName,
	))
	if s.character.GUID == eventData.MemberGUID {
		s.character.GuildID = 0
	}

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

	msgType := ChatTypeGuild
	if eventData.ForOfficers {
		msgType = ChatTypeOfficer
	}

	s.sendAzerothCorePlayerChat(msgType, eventData.Language, eventData.RealmID, eventData.SenderGUID, eventData.SenderName, 0, "", eventData.Msg, eventData.SenderChatTag)

	return nil
}

func (s *GameSession) HandleGuildInviteAccept(ctx context.Context, _ *packet.Packet) error {
	inviteResp, err := s.guildServiceClient.InviteAccepted(ctx, &pbGuild.InviteAcceptedParams{
		Api:     root.Ver,
		RealmID: s.guildHomeRealmID(),
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
		AllowCrossFaction: root.AllowTwoSideInteractionGuild,
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
		RealmID: s.guildHomeRealmID(),
		Leaver:  s.character.GUID,
	})
	if err != nil {
		return fmt.Errorf("can't leave the guild, err: %w", err)
	}

	s.character.GuildID = 0

	return nil
}

func (s *GameSession) HandleGuildKick(ctx context.Context, p *packet.Packet) error {
	target, err := s.guildCharacterByName(ctx, p.Reader().String())
	if err != nil {
		return err
	}

	if target == nil {
		s.SendSysMessage("Player not found")
		return nil
	}

	_, err = s.guildServiceClient.Kick(ctx, &pbGuild.KickParams{
		Api:     root.Ver,
		RealmID: s.guildHomeRealmID(),
		Kicker:  s.character.GUID,
		Target:  target.CharGUID,
	})
	if err != nil {
		return fmt.Errorf("can't kick player from the guild, err: %w", err)
	}

	return nil
}

func (s *GameSession) HandleGuildSetMessageOfTheDay(ctx context.Context, p *packet.Packet) error {
	_, err := s.guildServiceClient.SetMessageOfTheDay(ctx, &pbGuild.SetMessageOfTheDayParams{
		Api:             root.Ver,
		RealmID:         s.guildHomeRealmID(),
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

	target, err := s.guildCharacterByName(ctx, targetName)
	if err != nil {
		return err
	}

	if target == nil {
		s.SendSysMessage("Player not found")
		return nil
	}

	_, err = s.guildServiceClient.SetMemberPublicNote(ctx, &pbGuild.SetNoteParams{
		Api:         root.Ver,
		RealmID:     s.guildHomeRealmID(),
		ChangerGUID: s.character.GUID,
		TargetGUID:  target.CharGUID,
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

	target, err := s.guildCharacterByName(ctx, targetName)
	if err != nil {
		return err
	}

	if target == nil {
		s.SendSysMessage("Player not found")
		return nil
	}

	_, err = s.guildServiceClient.SetMemberOfficerNote(ctx, &pbGuild.SetNoteParams{
		Api:         root.Ver,
		RealmID:     s.guildHomeRealmID(),
		ChangerGUID: s.character.GUID,
		TargetGUID:  target.CharGUID,
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
		RealmID:     s.guildHomeRealmID(),
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
	bankTabRights := make([]*pbGuild.RankUpdateParams_BankTabRight, 0, guildBankMaxTabs)

	for tabID := uint32(0); tabID < guildBankMaxTabs; tabID++ {
		tabFlags := reader.Uint32()
		withdrawItemLimit := reader.Uint32()
		bankTabRights = append(bankTabRights, &pbGuild.RankUpdateParams_BankTabRight{
			TabID:             tabID,
			Flags:             tabFlags,
			WithdrawItemLimit: withdrawItemLimit,
		})
	}

	_, err := s.guildServiceClient.UpdateRank(ctx, &pbGuild.RankUpdateParams{
		Api:           root.Ver,
		RealmID:       s.guildHomeRealmID(),
		ChangerGUID:   s.character.GUID,
		Rank:          rankID,
		RankName:      name,
		Rights:        rights,
		MoneyPerDay:   withdrawGoldLimit,
		BankTabRights: bankTabRights,
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
		RealmID:     s.guildHomeRealmID(),
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
		RealmID:     s.guildHomeRealmID(),
		ChangerGUID: s.character.GUID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *GameSession) HandleGuildPromote(ctx context.Context, p *packet.Packet) error {
	target, err := s.guildCharacterByName(ctx, p.Reader().String())
	if err != nil {
		return err
	}

	if target == nil {
		s.SendSysMessage("Player not found")
		return nil
	}

	_, err = s.guildServiceClient.PromoteMember(ctx, &pbGuild.PromoteDemoteParams{
		Api:         root.Ver,
		RealmID:     s.guildHomeRealmID(),
		ChangerGUID: s.character.GUID,
		TargetGUID:  target.CharGUID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *GameSession) HandleGuildDemote(ctx context.Context, p *packet.Packet) error {
	target, err := s.guildCharacterByName(ctx, p.Reader().String())
	if err != nil {
		return err
	}

	if target == nil {
		s.SendSysMessage("Player not found")
		return nil
	}

	_, err = s.guildServiceClient.DemoteMember(ctx, &pbGuild.PromoteDemoteParams{
		Api:         root.Ver,
		RealmID:     s.guildHomeRealmID(),
		ChangerGUID: s.character.GUID,
		TargetGUID:  target.CharGUID,
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
		RealmID: s.guildHomeRealmID(),
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

func (s *GameSession) InterceptGuildCommandResult(_ context.Context, p *packet.Packet) error {
	if s.gameSocket != nil {
		s.gameSocket.WriteChannel() <- p
	}

	reader := p.Reader()
	command := reader.Int32()
	name := reader.String()
	result := reader.Int32()
	if reader.Error() != nil {
		s.pendingGuildCreate = nil
		return nil
	}

	if command == guildCommandCreate && result == guildCommandResultSuccess {
		s.pendingGuildCreate = &pendingGuildCreateState{name: name}
		return nil
	}

	s.pendingGuildCreate = nil
	return nil
}

func (s *GameSession) InterceptTurnInPetitionResults(_ context.Context, p *packet.Packet) error {
	if s.gameSocket != nil {
		s.gameSocket.WriteChannel() <- p
	}

	reader := p.Reader()
	result := reader.Uint32()
	if reader.Error() != nil {
		s.pendingGuildCreate = nil
		return nil
	}

	if result == petitionTurnOK && s.pendingGuildCreate != nil && s.character != nil && s.eventsProducer != nil {
		err := s.eventsProducer.GuildCreated(&events.GWEventGuildCreatedPayload{
			RealmID:    s.guildHomeRealmID(),
			LeaderGUID: s.character.GUID,
			GuildName:  s.pendingGuildCreate.name,
		})
		if err != nil {
			s.logger.Err(err).Msg("can't send guild created event")
		}
	}

	s.pendingGuildCreate = nil
	return nil
}

func (s *GameSession) sendTurnInPetitionResult(result uint32) {
	resp := packet.NewWriterWithSize(packet.SMsgTurnInPetitionResults, 4)
	resp.Uint32(result)
	s.gameSocket.Send(resp)
}

func (s *GameSession) guildCharacterByName(ctx context.Context, name string) (*pbChar.CharacterByNameResponse_Char, error) {
	resp, err := s.charServiceClient.CharacterByName(ctx, &pbChar.CharacterByNameRequest{
		Api:           root.Ver,
		RealmID:       s.guildHomeRealmID(),
		CharacterName: name,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}

	return resp.GetCharacter(), nil
}

func (s *GameSession) HandleGuildPermissions(ctx context.Context, p *packet.Packet) error {
	if s.character.GuildID == 0 {
		// TODO: send proper message to the client
		return nil
	}

	guildResp, err := s.guildServiceClient.GetRosterInfo(ctx, &pbGuild.GetRosterInfoParams{
		Api:     root.Ver,
		RealmID: s.guildHomeRealmID(),
		GuildID: uint64(s.character.GuildID),
	})
	if err != nil {
		return err
	}

	resp := packet.NewWriterWithSize(packet.MsgGuildPermissions, 0)
	member, rank := currentGuildMemberRank(guildResp, s.character.GUID)
	if member != nil && rank != nil {
		resp.Uint32(rank.Id)
		resp.Int32(int32(rank.Flags))
		resp.Int32(remainingGuildBankMoney(rank, member))
		resp.Uint8(uint8(clampGuildBankTabs(rank, guildResp.Guild.PurchasedBankTabs)))
		writeGuildBankTabRemainingRights(resp, rank, member)
	}

	s.gameSocket.Send(resp)
	return nil
}

func (s *GameSession) HandleGuildBankMoneyWithdrawn(ctx context.Context, p *packet.Packet) error {
	if s.character.GuildID == 0 {
		return nil
	}

	guildResp, err := s.guildServiceClient.GetRosterInfo(ctx, &pbGuild.GetRosterInfoParams{
		Api:     root.Ver,
		RealmID: s.guildHomeRealmID(),
		GuildID: uint64(s.character.GuildID),
	})
	if err != nil {
		return err
	}

	member, rank := currentGuildMemberRank(guildResp, s.character.GUID)
	if member == nil || rank == nil {
		return nil
	}

	resp := packet.NewWriterWithSize(packet.MsgGuildBankMoneyWithdrawn, 4)
	resp.Int32(remainingGuildBankMoney(rank, member))
	s.gameSocket.Send(resp)
	return nil
}

func (s *GameSession) guildHomeRealmID() uint32 {
	// Guilds are home-realm scoped even when the player is routed through a
	// shared crossrealm map owner.
	return root.RealmID
}

func (s *GameSession) guildCanInviteRace(targetRace uint8) bool {
	if root.AllowTwoSideInteractionGuild {
		return true
	}
	if s == nil || s.character == nil {
		return false
	}

	return guildSameFactionByRace(s.character.Race, targetRace)
}

func guildSameFactionByRace(leftRace, rightRace uint8) bool {
	leftTeam, ok := guildRaceTeam(leftRace)
	if !ok {
		return false
	}
	rightTeam, ok := guildRaceTeam(rightRace)
	if !ok {
		return false
	}
	return leftTeam == rightTeam
}

func guildRaceTeam(race uint8) (wow.Team, bool) {
	if int(race) >= len(wow.DefaultRaces) {
		return 0, false
	}
	raceInfo := wow.DefaultRaces[race]
	if raceInfo.ID == 0 {
		return 0, false
	}
	return raceInfo.Team, true
}

func writeGuildBankTabRights(resp *packet.Writer, rank *pbGuild.GetRosterInfoResponse_Rank) {
	var rights [guildBankMaxTabs]*pbGuild.GetRosterInfoResponse_Rank_BankTabRight
	for _, right := range rank.BankTabRights {
		if right.TabID < guildBankMaxTabs {
			rights[right.TabID] = right
		}
	}

	for i := 0; i < guildBankMaxTabs; i++ {
		if rights[i] == nil {
			resp.Uint32(0)
			resp.Uint32(0)
			continue
		}

		resp.Uint32(rights[i].Flags)
		resp.Uint32(rights[i].WithdrawItemLimit)
	}
}

func writeGuildBankTabRemainingRights(resp *packet.Writer, rank *pbGuild.GetRosterInfoResponse_Rank, member *pbGuild.GetRosterInfoResponse_Member) {
	var rights [guildBankMaxTabs]*pbGuild.GetRosterInfoResponse_Rank_BankTabRight
	for _, right := range rank.BankTabRights {
		if right.TabID < guildBankMaxTabs {
			rights[right.TabID] = right
		}
	}

	for i := 0; i < guildBankMaxTabs; i++ {
		if rights[i] == nil {
			resp.Int32(0)
			resp.Int32(0)
			continue
		}

		resp.Int32(int32(rights[i].Flags))
		resp.Int32(remainingGuildBankTabSlots(rights[i], member, i))
	}
}

func currentGuildMemberRank(guildResp *pbGuild.GetRosterInfoResponse, memberGUID uint64) (*pbGuild.GetRosterInfoResponse_Member, *pbGuild.GetRosterInfoResponse_Rank) {
	if guildResp == nil || guildResp.Guild == nil {
		return nil, nil
	}

	var member *pbGuild.GetRosterInfoResponse_Member
	for _, candidate := range guildResp.Guild.Members {
		if candidate.Guid == memberGUID {
			member = candidate
			break
		}
	}
	if member == nil {
		return nil, nil
	}

	for _, rank := range guildResp.Guild.Ranks {
		if rank.Id == member.RankID {
			return member, rank
		}
	}

	return member, nil
}

func remainingGuildBankMoney(rank *pbGuild.GetRosterInfoResponse_Rank, member *pbGuild.GetRosterInfoResponse_Member) int32 {
	if rank.GoldLimit == guildWithdrawUnlimited {
		return -1
	}
	if rank.Flags&(guildRightWithdrawRepair|guildRightWithdrawGold) == 0 {
		return 0
	}

	return remainingGuildBankLimit(rank.GoldLimit, bankWithdrawAt(member, guildBankWithdrawMoneyIdx))
}

func remainingGuildBankTabSlots(right *pbGuild.GetRosterInfoResponse_Rank_BankTabRight, member *pbGuild.GetRosterInfoResponse_Member, tabID int) int32 {
	if right.WithdrawItemLimit == guildWithdrawUnlimited {
		return -1
	}
	if right.Flags&guildBankRightViewTab == 0 {
		return 0
	}

	return remainingGuildBankLimit(right.WithdrawItemLimit, bankWithdrawAt(member, tabID))
}

func remainingGuildBankLimit(limit uint32, used uint32) int32 {
	if used >= limit {
		return 0
	}

	return int32(limit - used)
}

func bankWithdrawAt(member *pbGuild.GetRosterInfoResponse_Member, index int) uint32 {
	if member == nil || index < 0 || index >= len(member.BankWithdraw) {
		return 0
	}

	return member.BankWithdraw[index]
}

func clampGuildBankTabs(rank *pbGuild.GetRosterInfoResponse_Rank, purchasedTabs uint32) uint32 {
	if purchasedTabs > guildBankMaxTabs {
		return guildBankMaxTabs
	}
	if purchasedTabs != 0 {
		return purchasedTabs
	}

	for _, right := range rank.BankTabRights {
		if right.TabID < guildBankMaxTabs && right.Flags != 0 {
			return right.TabID + 1
		}
	}

	return 0
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
