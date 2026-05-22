package session

import (
	"context"
	"strings"

	root "github.com/walkline/ToCloud9/apps/gateway"
	eBroadcaster "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	pbMM "github.com/walkline/ToCloud9/gen/matchmaking/pb"
	pbWorld "github.com/walkline/ToCloud9/gen/worldserver/pb"
	"github.com/walkline/ToCloud9/shared/events"
)

const (
	arenaTeamActionCreate uint32 = 0x00
	arenaTeamActionInvite uint32 = 0x01
	arenaTeamActionQuit   uint32 = 0x03

	arenaTeamErrorSuccess        uint32 = 0
	arenaTeamErrorInternal       uint32 = 0x01
	arenaTeamErrorAlreadyInTeam  uint32 = 0x03
	arenaTeamErrorAlreadyInvited uint32 = 0x05
	arenaTeamErrorInvalidName    uint32 = 0x06
	arenaTeamErrorNameExists     uint32 = 0x07
	arenaTeamErrorLeaderLeave    uint32 = 0x08
	arenaTeamErrorPermissions    uint32 = 0x08
	arenaTeamErrorPlayerNotFound uint32 = 0x0B
	arenaTeamErrorNotAllied      uint32 = 0x0C
	arenaTeamErrorIgnoringYou    uint32 = 0x13
	arenaTeamErrorTargetTooLow   uint32 = 0x15
	arenaTeamErrorTooManyMembers uint32 = 0x17
	arenaTeamErrorNotFound       uint32 = 0x1B
	arenaTeamErrorLocked         uint32 = 0x1E
)

func (s *GameSession) HandleArenaTeamPetitionTurnIn(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	petitionGUID := reader.Uint64()
	if reader.Error() != nil || s.character == nil || s.charServiceClient == nil {
		s.forwardPacketToWorldserver(p)
		return nil
	}

	petitionResp, err := s.charServiceClient.GetArenaTeamPetition(ctx, &pbChar.GetArenaTeamPetitionRequest{
		Api:          root.Ver,
		RealmID:      root.RealmID,
		PetitionGUID: petitionGUID,
	})
	if err != nil {
		return err
	}
	if petitionResp.GetStatus() == pbChar.GetArenaTeamPetitionResponse_NotFound ||
		petitionResp.GetStatus() == pbChar.GetArenaTeamPetitionResponse_NotArena ||
		petitionResp.GetPetition() == nil {
		s.forwardPacketToWorldserver(p)
		return nil
	}
	if petitionResp.GetStatus() != pbChar.GetArenaTeamPetitionResponse_Ok {
		s.sendArenaTeamCommandResult(arenaTeamActionCreate, "", "", arenaTeamErrorInternal)
		return nil
	}

	petition := petitionResp.GetPetition()
	background := reader.Uint32()
	icon := reader.Uint32()
	iconColor := reader.Uint32()
	border := reader.Uint32()
	borderColor := reader.Uint32()
	if reader.Error() != nil {
		s.sendArenaTeamCommandResult(arenaTeamActionCreate, petition.GetName(), "", arenaTeamErrorInternal)
		return nil
	}

	if s.gameServerGRPCClient == nil {
		s.sendArenaTeamCommandResult(arenaTeamActionCreate, petition.GetName(), "", arenaTeamErrorInternal)
		return nil
	}

	items, err := s.gameServerGRPCClient.GetPlayerItemsByGuids(ctx, &pbWorld.GetPlayerItemsByGuidsRequest{
		Api:        root.SupportedGameServerVer,
		PlayerGuid: s.character.GUID,
		Guids:      []uint64{petitionGUID},
	})
	if err != nil {
		return err
	}
	if len(items.GetItems()) != 1 {
		s.sendArenaTeamCommandResult(arenaTeamActionCreate, petition.GetName(), "", arenaTeamErrorInternal)
		return nil
	}

	startRating := root.EffectiveArenaStartRating()
	resp, err := s.charServiceClient.CreateArenaTeamFromPetition(ctx, &pbChar.CreateArenaTeamFromPetitionRequest{
		Api:                   root.Ver,
		RealmID:               root.RealmID,
		CaptainGUID:           s.character.GUID,
		PetitionGUID:          petitionGUID,
		ArenaType:             petition.GetArenaType(),
		BackgroundColor:       background,
		EmblemStyle:           icon,
		EmblemColor:           iconColor,
		BorderStyle:           border,
		BorderColor:           borderColor,
		StartRating:           startRating,
		StartPersonalRating:   root.EffectiveArenaStartPersonalRating(startRating),
		StartMatchmakerRating: root.ArenaStartMatchmakerRating,
	})
	if err != nil {
		return err
	}

	switch resp.GetStatus() {
	case pbChar.CreateArenaTeamFromPetitionResponse_Ok:
		removed, err := s.gameServerGRPCClient.RemoveItemsWithGuidsFromPlayer(ctx, &pbWorld.RemoveItemsWithGuidsFromPlayerRequest{
			Api:                root.SupportedGameServerVer,
			PlayerGuid:         s.character.GUID,
			Guids:              []uint64{petitionGUID},
			AssignToPlayerGuid: 0,
		})
		if err != nil {
			return err
		}
		if len(removed.GetUpdatedItemsGuids()) != 1 {
			s.sendArenaTeamCommandResult(arenaTeamActionCreate, petition.GetName(), "", arenaTeamErrorInternal)
			return nil
		}

		s.sendArenaTeamCommandResult(arenaTeamActionCreate, petition.GetName(), "", arenaTeamErrorSuccess)
		s.sendTurnInPetitionResult(petitionTurnOK)
	case pbChar.CreateArenaTeamFromPetitionResponse_NameExists:
		s.sendArenaTeamCommandResult(arenaTeamActionCreate, petition.GetName(), "", arenaTeamErrorNameExists)
	case pbChar.CreateArenaTeamFromPetitionResponse_AlreadyInTeam:
		s.sendArenaTeamCommandResult(arenaTeamActionCreate, petition.GetName(), "", arenaTeamErrorAlreadyInTeam)
	case pbChar.CreateArenaTeamFromPetitionResponse_NotEnoughSignatures:
		s.sendTurnInPetitionResult(petitionTurnNeedMoreSignatures)
	case pbChar.CreateArenaTeamFromPetitionResponse_InvalidName:
		s.sendArenaTeamCommandResult(arenaTeamActionCreate, petition.GetName(), "", arenaTeamErrorInvalidName)
	default:
		s.sendArenaTeamCommandResult(arenaTeamActionCreate, petition.GetName(), "", arenaTeamErrorInternal)
	}

	return nil
}

func (s *GameSession) HandleArenaTeamQuery(ctx context.Context, p *packet.Packet) error {
	teamID := p.Reader().Uint32()
	team, err := s.arenaTeamForGateway(ctx, teamID)
	if err != nil || team == nil {
		return err
	}

	s.sendArenaTeamQuery(team)
	s.sendArenaTeamStats(team)
	return nil
}

func (s *GameSession) HandleArenaTeamRoster(ctx context.Context, p *packet.Packet) error {
	teamID := p.Reader().Uint32()
	team, err := s.arenaTeamForGateway(ctx, teamID)
	if err != nil || team == nil {
		return err
	}

	s.sendArenaTeamRoster(team)
	return nil
}

func (s *GameSession) HandleArenaTeamInvite(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	teamID := reader.Uint32()
	targetName := reader.String()
	if reader.Error() != nil || s.character == nil {
		return nil
	}

	targetResp, err := s.charServiceClient.CharacterOnlineByName(ctx, &pbChar.CharacterOnlineByNameRequest{
		Api:           root.Ver,
		RealmID:       root.RealmID,
		CharacterName: targetName,
	})
	if err != nil {
		return err
	}
	if targetResp.GetCharacter() == nil || targetResp.GetCharacter().GetRealmID() != root.RealmID {
		s.sendArenaTeamCommandResult(arenaTeamActionCreate, "", targetName, arenaTeamErrorPlayerNotFound)
		return nil
	}
	target := targetResp.GetCharacter()
	if root.MaxPlayerLevel > 0 && target.GetCharLvl() < root.MaxPlayerLevel {
		s.sendArenaTeamCommandResult(arenaTeamActionCreate, "", target.GetCharName(), arenaTeamErrorTargetTooLow)
		return nil
	}
	if !s.arenaCanInviteRace(uint8(target.GetCharRace())) {
		s.sendArenaTeamCommandResult(arenaTeamActionInvite, "", "", arenaTeamErrorNotAllied)
		return nil
	}

	resp, err := s.charServiceClient.InviteArenaTeamMember(ctx, &pbChar.InviteArenaTeamMemberRequest{
		Api:         root.Ver,
		RealmID:     root.RealmID,
		ArenaTeamID: teamID,
		InviterGUID: s.character.GUID,
		InviterName: s.character.Name,
		TargetGUID:  target.GetCharGUID(),
		TargetName:  target.GetCharName(),
	})
	if err != nil {
		return err
	}

	if resp.GetStatus() != pbChar.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_OK {
		s.sendArenaTeamCommandResult(arenaTeamActionInvite, "", target.GetCharName(), arenaTeamMutationStatusToNativeError(resp.GetStatus()))
		return nil
	}

	s.sendArenaTeamCommandResult(arenaTeamActionInvite, resp.GetTeam().GetName(), target.GetCharName(), arenaTeamErrorSuccess)
	return nil
}

func (s *GameSession) HandleArenaTeamAccept(ctx context.Context, _ *packet.Packet) error {
	if s.character == nil {
		return nil
	}

	resp, err := s.charServiceClient.AcceptArenaTeamInvite(ctx, &pbChar.AcceptArenaTeamInviteRequest{
		Api:        root.Ver,
		RealmID:    root.RealmID,
		PlayerGUID: s.character.GUID,
		PlayerName: s.character.Name,
	})
	if err != nil {
		return err
	}
	if resp.GetStatus() != pbChar.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_OK {
		s.sendArenaTeamCommandResult(arenaTeamActionCreate, "", "", arenaTeamMutationStatusToNativeError(resp.GetStatus()))
		return nil
	}

	team := resp.GetTeam()
	s.sendArenaTeamQuery(team)
	s.sendArenaTeamRoster(team)
	s.sendArenaTeamStats(team)
	return nil
}

func (s *GameSession) HandleArenaTeamDecline(ctx context.Context, _ *packet.Packet) error {
	if s.character == nil {
		return nil
	}

	_, err := s.charServiceClient.DeclineArenaTeamInvite(ctx, &pbChar.DeclineArenaTeamInviteRequest{
		Api:        root.Ver,
		RealmID:    root.RealmID,
		PlayerGUID: s.character.GUID,
	})
	return err
}

func (s *GameSession) HandleArenaTeamLeave(ctx context.Context, p *packet.Packet) error {
	teamID := p.Reader().Uint32()
	team, err := s.arenaTeamForGateway(ctx, teamID)
	if err != nil || team == nil || s.character == nil {
		return err
	}
	if locked, err := s.arenaTeamMutationLocked(ctx, team); err != nil {
		return err
	} else if locked {
		s.sendArenaTeamCommandResult(arenaTeamActionQuit, "", "", arenaTeamErrorLocked)
		return nil
	}

	if team.GetCaptainGUID() == s.character.GUID {
		if len(team.GetMembers()) > 1 {
			s.sendArenaTeamCommandResult(arenaTeamActionQuit, "", "", arenaTeamErrorLeaderLeave)
			return nil
		}
		resp, err := s.charServiceClient.DisbandArenaTeam(ctx, &pbChar.DisbandArenaTeamRequest{
			Api:         root.Ver,
			RealmID:     root.RealmID,
			ArenaTeamID: teamID,
			ActorGUID:   s.character.GUID,
		})
		if err != nil {
			return err
		}
		if resp.GetStatus() != pbChar.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_OK {
			s.sendArenaTeamCommandResult(arenaTeamActionCreate, team.GetName(), "", arenaTeamMutationStatusToNativeError(resp.GetStatus()))
			return nil
		}
		return nil
	}

	resp, err := s.charServiceClient.RemoveArenaTeamMember(ctx, &pbChar.RemoveArenaTeamMemberRequest{
		Api:         root.Ver,
		RealmID:     root.RealmID,
		ArenaTeamID: teamID,
		PlayerGUID:  s.character.GUID,
		ActorGUID:   s.character.GUID,
	})
	if err != nil {
		return err
	}
	if resp.GetStatus() != pbChar.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_OK {
		s.sendArenaTeamCommandResult(arenaTeamActionQuit, team.GetName(), "", arenaTeamMutationStatusToNativeError(resp.GetStatus()))
		return nil
	}

	s.sendArenaTeamCommandResult(arenaTeamActionQuit, team.GetName(), "", arenaTeamErrorSuccess)
	return nil
}

func (s *GameSession) HandleArenaTeamRemove(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	teamID := reader.Uint32()
	targetName := reader.String()
	if reader.Error() != nil || s.character == nil {
		return nil
	}

	team, err := s.arenaTeamForGateway(ctx, teamID)
	if err != nil || team == nil {
		return err
	}
	if locked, err := s.arenaTeamMutationLocked(ctx, team); err != nil {
		return err
	} else if locked {
		s.sendArenaTeamCommandResult(arenaTeamActionQuit, "", "", arenaTeamErrorLocked)
		return nil
	}
	target := arenaTeamMemberByName(team, targetName)
	if target == nil {
		s.sendArenaTeamCommandResult(arenaTeamActionCreate, "", targetName, arenaTeamErrorPlayerNotFound)
		return nil
	}

	resp, err := s.charServiceClient.RemoveArenaTeamMember(ctx, &pbChar.RemoveArenaTeamMemberRequest{
		Api:         root.Ver,
		RealmID:     root.RealmID,
		ArenaTeamID: teamID,
		PlayerGUID:  target.GetPlayerGUID(),
		ActorGUID:   s.character.GUID,
	})
	if err != nil {
		return err
	}
	if resp.GetStatus() != pbChar.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_OK {
		s.sendArenaTeamCommandResult(arenaTeamActionCreate, team.GetName(), target.GetName(), arenaTeamMutationStatusToNativeError(resp.GetStatus()))
		return nil
	}

	return nil
}

func (s *GameSession) HandleArenaTeamDisband(ctx context.Context, p *packet.Packet) error {
	teamID := p.Reader().Uint32()
	team, err := s.arenaTeamForGateway(ctx, teamID)
	if err != nil || team == nil || s.character == nil {
		return err
	}
	if locked, err := s.arenaTeamMutationLocked(ctx, team); err != nil {
		return err
	} else if locked {
		return nil
	}

	resp, err := s.charServiceClient.DisbandArenaTeam(ctx, &pbChar.DisbandArenaTeamRequest{
		Api:         root.Ver,
		RealmID:     root.RealmID,
		ArenaTeamID: teamID,
		ActorGUID:   s.character.GUID,
	})
	if err != nil {
		return err
	}
	if resp.GetStatus() != pbChar.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_OK {
		s.sendArenaTeamCommandResult(arenaTeamActionCreate, team.GetName(), "", arenaTeamMutationStatusToNativeError(resp.GetStatus()))
		return nil
	}

	return nil
}

func (s *GameSession) HandleArenaTeamLeader(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	teamID := reader.Uint32()
	targetName := reader.String()
	if reader.Error() != nil || s.character == nil {
		return nil
	}

	team, err := s.arenaTeamForGateway(ctx, teamID)
	if err != nil || team == nil {
		return err
	}
	target := arenaTeamMemberByName(team, targetName)
	if target == nil {
		s.sendArenaTeamCommandResult(arenaTeamActionCreate, "", targetName, arenaTeamErrorPlayerNotFound)
		return nil
	}

	resp, err := s.charServiceClient.SetArenaTeamCaptain(ctx, &pbChar.SetArenaTeamCaptainRequest{
		Api:         root.Ver,
		RealmID:     root.RealmID,
		ArenaTeamID: teamID,
		CaptainGUID: target.GetPlayerGUID(),
		ActorGUID:   s.character.GUID,
	})
	if err != nil {
		return err
	}
	if resp.GetStatus() != pbChar.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_OK {
		s.sendArenaTeamCommandResult(arenaTeamActionCreate, team.GetName(), target.GetName(), arenaTeamMutationStatusToNativeError(resp.GetStatus()))
		return nil
	}

	return nil
}

func (s *GameSession) HandleEventArenaTeamInviteCreated(_ context.Context, e *eBroadcaster.Event) error {
	payload := e.Payload.(*events.CharEventArenaTeamInviteCreatedPayload)
	resp := packet.NewWriterWithSize(packet.SMsgArenaTeamInvite, 0)
	resp.String(payload.InviterName)
	resp.String(payload.TeamName)
	s.gameSocket.Send(resp)
	return nil
}

func (s *GameSession) HandleEventArenaTeamNativeEvent(_ context.Context, e *eBroadcaster.Event) error {
	payload := e.Payload.(*events.CharEventArenaTeamNativeEventPayload)
	s.sendArenaTeamEvent(payload.Event, payload.EventGUID, payload.Args...)
	return nil
}

func (s *GameSession) arenaCanInviteRace(targetRace uint8) bool {
	if root.AllowTwoSideInteractionArena {
		return true
	}
	if s == nil || s.character == nil {
		return false
	}

	return guildSameFactionByRace(s.character.Race, targetRace)
}

func (s *GameSession) arenaTeamMutationLocked(ctx context.Context, team *pbChar.ArenaTeamData) (bool, error) {
	if s.matchmakingServiceClient == nil || team == nil {
		return false, nil
	}

	for _, member := range team.GetMembers() {
		resp, err := s.matchmakingServiceClient.BattlegroundQueueDataForPlayer(ctx, &pbMM.BattlegroundQueueDataForPlayerRequest{
			Api:        root.SupportedMatchmakingServiceVer,
			RealmID:    root.RealmID,
			PlayerGUID: member.GetPlayerGUID(),
		})
		if err != nil {
			return false, err
		}
		for _, slot := range resp.GetSlots() {
			if isArenaBattlegroundTypeID(slot.GetBgTypeID()) && slot.GetStatus() != pbMM.PlayerQueueStatus_NotInQueue {
				return true, nil
			}
		}
	}

	return false, nil
}

func (s *GameSession) arenaTeamForGateway(ctx context.Context, teamID uint32) (*pbChar.ArenaTeamData, error) {
	if s.charServiceClient == nil {
		return nil, nil
	}

	resp, err := s.charServiceClient.GetArenaTeam(ctx, &pbChar.GetArenaTeamRequest{
		Api:         root.Ver,
		RealmID:     root.RealmID,
		ArenaTeamID: teamID,
	})
	if err != nil {
		return nil, err
	}
	if resp.GetStatus() != pbChar.GetArenaTeamResponse_Ok || resp.GetTeam() == nil {
		return nil, nil
	}
	return resp.GetTeam(), nil
}

func (s *GameSession) sendArenaTeamCommandResult(action uint32, team string, player string, errorID uint32) {
	resp := packet.NewWriterWithSize(packet.SMsgArenaTeamCommandResult, 0)
	resp.Uint32(action)
	resp.String(team)
	resp.String(player)
	resp.Uint32(errorID)
	s.gameSocket.Send(resp)
}

func (s *GameSession) sendArenaTeamQuery(team *pbChar.ArenaTeamData) {
	resp := packet.NewWriterWithSize(packet.SMsgArenaTeamQueryResponse, 0)
	resp.Uint32(team.GetArenaTeamID())
	resp.String(team.GetName())
	resp.Uint32(team.GetType())
	resp.Uint32(team.GetBackgroundColor())
	resp.Uint32(team.GetEmblemStyle())
	resp.Uint32(team.GetEmblemColor())
	resp.Uint32(team.GetBorderStyle())
	resp.Uint32(team.GetBorderColor())
	s.gameSocket.Send(resp)
}

func (s *GameSession) sendArenaTeamStats(team *pbChar.ArenaTeamData) {
	resp := packet.NewWriterWithSize(packet.SMsgArenaTeamStats, 0)
	resp.Uint32(team.GetArenaTeamID())
	resp.Uint32(team.GetRating())
	resp.Uint32(team.GetWeekGames())
	resp.Uint32(team.GetWeekWins())
	resp.Uint32(team.GetSeasonGames())
	resp.Uint32(team.GetSeasonWins())
	resp.Uint32(team.GetRank())
	s.gameSocket.Send(resp)
}

func (s *GameSession) sendArenaTeamRoster(team *pbChar.ArenaTeamData) {
	resp := packet.NewWriterWithSize(packet.SMsgArenaTeamRoster, 0)
	resp.Uint32(team.GetArenaTeamID())
	resp.Uint8(0)
	resp.Uint32(uint32(len(team.GetMembers())))
	resp.Uint32(team.GetType())

	for _, member := range team.GetMembers() {
		resp.Uint64(member.GetPlayerGUID())
		resp.Bool(member.GetOnline())
		resp.String(member.GetName())
		if member.GetPlayerGUID() == team.GetCaptainGUID() {
			resp.Uint32(0)
		} else {
			resp.Uint32(1)
		}
		resp.Uint8(uint8(member.GetLevel()))
		resp.Uint8(uint8(member.GetClass()))
		resp.Uint32(member.GetWeekGames())
		resp.Uint32(member.GetWeekWins())
		resp.Uint32(member.GetSeasonGames())
		resp.Uint32(member.GetSeasonWins())
		resp.Uint32(member.GetPersonalRating())
	}

	s.gameSocket.Send(resp)
}

func (s *GameSession) sendArenaTeamEvent(event uint8, guid uint64, args ...string) {
	resp := packet.NewWriterWithSize(packet.SMsgArenaTeamEvent, 0)
	resp.Uint8(event)
	resp.Uint8(uint8(len(args)))
	for _, arg := range args {
		resp.String(arg)
	}
	if guid > 0 {
		resp.Uint64(guid)
	}
	s.gameSocket.Send(resp)
}

func arenaTeamMemberByName(team *pbChar.ArenaTeamData, name string) *pbChar.ArenaTeamMemberData {
	for _, member := range team.GetMembers() {
		if strings.EqualFold(member.GetName(), name) {
			return member
		}
	}
	return nil
}

func arenaTeamMutationStatusToNativeError(status pbChar.ArenaTeamMutationStatus) uint32 {
	switch status {
	case pbChar.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_OK:
		return arenaTeamErrorSuccess
	case pbChar.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_NOT_FOUND:
		return arenaTeamErrorNotFound
	case pbChar.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_MEMBER_MISMATCH:
		return arenaTeamErrorPlayerNotFound
	case pbChar.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_INVALID_TYPE:
		return arenaTeamErrorNotFound
	case pbChar.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_NAME_EXISTS:
		return arenaTeamErrorNameExists
	case pbChar.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_ALREADY_IN_TEAM:
		return arenaTeamErrorAlreadyInTeam
	case pbChar.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_ROSTER_FULL:
		return arenaTeamErrorTooManyMembers
	case pbChar.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_INVALID_NAME:
		return arenaTeamErrorInvalidName
	case pbChar.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_PERMISSION_DENIED:
		return arenaTeamErrorPermissions
	case pbChar.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_LEADER_LEAVE:
		return arenaTeamErrorLeaderLeave
	case pbChar.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_ALREADY_INVITED:
		return arenaTeamErrorAlreadyInvited
	case pbChar.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_IGNORING_YOU:
		return arenaTeamErrorIgnoringYou
	default:
		return arenaTeamErrorInternal
	}
}
