package server

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/charserver/repo"
	"github.com/walkline/ToCloud9/gen/characters/pb"
	"github.com/walkline/ToCloud9/shared/events"
)

const (
	arenaTeamNativeEventJoin          uint8 = 3
	arenaTeamNativeEventLeave         uint8 = 4
	arenaTeamNativeEventRemove        uint8 = 5
	arenaTeamNativeEventLeaderChanged uint8 = 7
	arenaTeamNativeEventDisbanded     uint8 = 8
)

func (c *CharServer) ArenaTeamQueueDataForRatedArena(ctx context.Context, request *pb.ArenaTeamQueueDataForRatedArenaRequest) (*pb.ArenaTeamQueueDataForRatedArenaResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint32("realmID", request.GetRealmID()).
			Uint64("leaderGUID", request.GetLeaderGUID()).
			Uint32("arenaType", request.GetArenaType()).
			Int("playerCount", len(request.GetPlayerGUIDs())).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled rated arena team queue data request")
	}(time.Now())

	if c.arenaTeams == nil {
		return &pb.ArenaTeamQueueDataForRatedArenaResponse{
			Api:    ver,
			Status: pb.ArenaTeamQueueDataForRatedArenaResponse_Failed,
		}, nil
	}

	data, err := c.arenaTeams.QueueDataForRatedArena(
		ctx,
		request.GetRealmID(),
		request.GetLeaderGUID(),
		request.GetPlayerGUIDs(),
		uint8(request.GetArenaType()),
		request.GetStartMatchmakerRating(),
	)
	if err != nil {
		status, handled := arenaTeamQueueStatusFromError(err)
		if handled {
			return &pb.ArenaTeamQueueDataForRatedArenaResponse{
				Api:    ver,
				Status: status,
			}, nil
		}
		return nil, err
	}

	return &pb.ArenaTeamQueueDataForRatedArenaResponse{
		Api:                     ver,
		Status:                  pb.ArenaTeamQueueDataForRatedArenaResponse_Ok,
		ArenaTeamID:             data.ArenaTeamID,
		TeamRating:              data.TeamRating,
		MatchmakerRating:        data.MatchmakerRating,
		PreviousOpponentsTeamID: data.PreviousOpponentsTeamID,
	}, nil
}

func (c *CharServer) CreateArenaTeamFromPetition(ctx context.Context, request *pb.CreateArenaTeamFromPetitionRequest) (*pb.CreateArenaTeamFromPetitionResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint32("realmID", request.GetRealmID()).
			Uint64("captainGUID", request.GetCaptainGUID()).
			Uint64("petitionGUID", request.GetPetitionGUID()).
			Uint32("arenaType", request.GetArenaType()).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled arena team create from petition request")
	}(time.Now())

	if c.arenaTeams == nil {
		return &pb.CreateArenaTeamFromPetitionResponse{
			Api:    ver,
			Status: pb.CreateArenaTeamFromPetitionResponse_Failed,
		}, nil
	}

	result, err := c.arenaTeams.CreateFromPetition(ctx, repo.ArenaTeamCreateFromPetitionRequest{
		RealmID:               request.GetRealmID(),
		CaptainGUID:           request.GetCaptainGUID(),
		PetitionGUID:          request.GetPetitionGUID(),
		ArenaType:             uint8(request.GetArenaType()),
		BackgroundColor:       request.GetBackgroundColor(),
		EmblemStyle:           uint8(request.GetEmblemStyle()),
		EmblemColor:           request.GetEmblemColor(),
		BorderStyle:           uint8(request.GetBorderStyle()),
		BorderColor:           request.GetBorderColor(),
		StartRating:           request.GetStartRating(),
		StartPersonalRating:   request.GetStartPersonalRating(),
		StartMatchmakerRating: request.GetStartMatchmakerRating(),
	})
	if err != nil {
		status, handled := arenaTeamCreateStatusFromError(err)
		if handled {
			return &pb.CreateArenaTeamFromPetitionResponse{
				Api:    ver,
				Status: status,
			}, nil
		}
		return nil, err
	}

	return &pb.CreateArenaTeamFromPetitionResponse{
		Api:         ver,
		Status:      pb.CreateArenaTeamFromPetitionResponse_Ok,
		ArenaTeamID: result.ArenaTeamID,
	}, nil
}

func (c *CharServer) GetArenaTeamPetition(ctx context.Context, request *pb.GetArenaTeamPetitionRequest) (*pb.GetArenaTeamPetitionResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint32("realmID", request.GetRealmID()).
			Uint64("petitionGUID", request.GetPetitionGUID()).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled arena team petition lookup request")
	}(time.Now())

	if c.arenaTeams == nil {
		return &pb.GetArenaTeamPetitionResponse{Api: ver, Status: pb.GetArenaTeamPetitionResponse_Failed}, nil
	}

	petition, err := c.arenaTeams.GetPetition(ctx, request.GetRealmID(), request.GetPetitionGUID())
	if err != nil {
		switch {
		case errors.Is(err, repo.ErrArenaTeamNotFound):
			return &pb.GetArenaTeamPetitionResponse{Api: ver, Status: pb.GetArenaTeamPetitionResponse_NotFound}, nil
		case errors.Is(err, repo.ErrArenaTeamInvalidType):
			return &pb.GetArenaTeamPetitionResponse{Api: ver, Status: pb.GetArenaTeamPetitionResponse_NotArena}, nil
		default:
			return nil, err
		}
	}

	return &pb.GetArenaTeamPetitionResponse{
		Api:    ver,
		Status: pb.GetArenaTeamPetitionResponse_Ok,
		Petition: &pb.ArenaTeamPetitionData{
			PetitionGUID: petition.PetitionGUID,
			PetitionID:   petition.PetitionID,
			OwnerGUID:    petition.OwnerGUID,
			Name:         petition.Name,
			ArenaType:    uint32(petition.ArenaType),
			Signatures:   petition.Signatures,
		},
	}, nil
}

func (c *CharServer) GetArenaTeam(ctx context.Context, request *pb.GetArenaTeamRequest) (*pb.GetArenaTeamResponse, error) {
	defer arenaTeamMutationLog("get team", request.GetRealmID(), request.GetArenaTeamID(), time.Now())

	if c.arenaTeams == nil {
		return &pb.GetArenaTeamResponse{Api: ver, Status: pb.GetArenaTeamResponse_Failed}, nil
	}

	team, err := c.arenaTeams.GetTeam(ctx, request.GetRealmID(), request.GetArenaTeamID())
	if err != nil {
		if errors.Is(err, repo.ErrArenaTeamNotFound) {
			return &pb.GetArenaTeamResponse{Api: ver, Status: pb.GetArenaTeamResponse_NotFound}, nil
		}
		return nil, err
	}

	return &pb.GetArenaTeamResponse{
		Api:    ver,
		Status: pb.GetArenaTeamResponse_Ok,
		Team:   c.arenaTeamData(ctx, request.GetRealmID(), team),
	}, nil
}

func (c *CharServer) InviteArenaTeamMember(ctx context.Context, request *pb.InviteArenaTeamMemberRequest) (*pb.InviteArenaTeamMemberResponse, error) {
	defer arenaTeamMutationLog("invite member", request.GetRealmID(), request.GetArenaTeamID(), time.Now())

	if c.arenaTeams == nil {
		return &pb.InviteArenaTeamMemberResponse{Api: ver, Status: pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_FAILED}, nil
	}

	team, err := c.arenaTeams.GetTeam(ctx, request.GetRealmID(), request.GetArenaTeamID())
	if err != nil {
		status, handled := arenaTeamMutationStatusFromError(err)
		if handled {
			return &pb.InviteArenaTeamMemberResponse{Api: ver, Status: status}, nil
		}
		return nil, err
	}

	if !arenaTeamHasMemberData(team, request.GetInviterGUID()) || team.CaptainGUID != request.GetInviterGUID() {
		return &pb.InviteArenaTeamMemberResponse{Api: ver, Status: pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_PERMISSION_DENIED}, nil
	}
	if arenaTeamHasMemberData(team, request.GetTargetGUID()) {
		return &pb.InviteArenaTeamMemberResponse{Api: ver, Status: pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_ALREADY_IN_TEAM}, nil
	}
	if ignoring, err := c.arenaTeamTargetIgnoresInviter(ctx, request.GetRealmID(), request.GetTargetGUID(), request.GetInviterGUID()); err != nil {
		return nil, err
	} else if ignoring {
		return &pb.InviteArenaTeamMemberResponse{Api: ver, Status: pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_IGNORING_YOU}, nil
	}
	if _, found, err := c.arenaTeams.MemberTeamForType(ctx, request.GetRealmID(), request.GetTargetGUID(), team.Type); err != nil {
		return nil, err
	} else if found {
		return &pb.InviteArenaTeamMemberResponse{Api: ver, Status: pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_ALREADY_IN_TEAM}, nil
	}
	if len(team.Members) >= int(team.Type)*2 {
		return &pb.InviteArenaTeamMemberResponse{Api: ver, Status: pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_ROSTER_FULL}, nil
	}

	inviteKey := arenaTeamInviteKey{realmID: request.GetRealmID(), playerGUID: request.GetTargetGUID()}
	c.arenaInvitesMu.Lock()
	if _, ok := c.arenaInvites[inviteKey]; ok {
		c.arenaInvitesMu.Unlock()
		return &pb.InviteArenaTeamMemberResponse{Api: ver, Status: pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_ALREADY_INVITED}, nil
	}
	c.arenaInvites[inviteKey] = arenaTeamInvite{
		arenaTeamID: request.GetArenaTeamID(),
		inviterGUID: request.GetInviterGUID(),
		inviterName: request.GetInviterName(),
		teamName:    team.Name,
	}
	c.arenaInvitesMu.Unlock()

	if c.eventsProducer != nil {
		if err = c.eventsProducer.ArenaTeamInviteCreated(&events.CharEventArenaTeamInviteCreatedPayload{
			RealmID:     request.GetRealmID(),
			TargetGUID:  request.GetTargetGUID(),
			TargetName:  request.GetTargetName(),
			InviterGUID: request.GetInviterGUID(),
			InviterName: request.GetInviterName(),
			ArenaTeamID: request.GetArenaTeamID(),
			TeamName:    team.Name,
		}); err != nil {
			return nil, err
		}
	}

	return &pb.InviteArenaTeamMemberResponse{
		Api:    ver,
		Status: pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_OK,
		Team:   c.arenaTeamData(ctx, request.GetRealmID(), team),
	}, nil
}

func (c *CharServer) AcceptArenaTeamInvite(ctx context.Context, request *pb.AcceptArenaTeamInviteRequest) (*pb.AcceptArenaTeamInviteResponse, error) {
	if c.arenaTeams == nil {
		return &pb.AcceptArenaTeamInviteResponse{Api: ver, Status: pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_FAILED}, nil
	}

	key := arenaTeamInviteKey{realmID: request.GetRealmID(), playerGUID: request.GetPlayerGUID()}
	c.arenaInvitesMu.Lock()
	invite, ok := c.arenaInvites[key]
	if ok {
		delete(c.arenaInvites, key)
	}
	c.arenaInvitesMu.Unlock()
	if !ok {
		return &pb.AcceptArenaTeamInviteResponse{Api: ver, Status: pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_NOT_FOUND}, nil
	}

	team, err := c.arenaTeams.GetTeam(ctx, request.GetRealmID(), invite.arenaTeamID)
	if err != nil {
		return nil, err
	}

	personalRating := request.GetPersonalRating()
	if personalRating == 0 {
		personalRating = team.Rating
	}

	if err := c.arenaTeams.AddMember(ctx, request.GetRealmID(), invite.arenaTeamID, request.GetPlayerGUID(), personalRating); err != nil {
		status, handled := arenaTeamMutationStatusFromError(err)
		if handled {
			return &pb.AcceptArenaTeamInviteResponse{Api: ver, Status: status}, nil
		}
		return nil, err
	}

	team, err = c.arenaTeams.GetTeam(ctx, request.GetRealmID(), invite.arenaTeamID)
	if err != nil {
		return nil, err
	}

	c.publishArenaTeamNativeEvent(
		request.GetRealmID(),
		team,
		arenaTeamNativeEventJoin,
		request.GetPlayerGUID(),
		request.GetPlayerName(),
		team.Name,
	)

	return &pb.AcceptArenaTeamInviteResponse{
		Api:    ver,
		Status: pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_OK,
		Team:   c.arenaTeamData(ctx, request.GetRealmID(), team),
	}, nil
}

func (c *CharServer) DeclineArenaTeamInvite(_ context.Context, request *pb.DeclineArenaTeamInviteRequest) (*pb.ArenaTeamMutationResponse, error) {
	key := arenaTeamInviteKey{realmID: request.GetRealmID(), playerGUID: request.GetPlayerGUID()}
	c.arenaInvitesMu.Lock()
	delete(c.arenaInvites, key)
	c.arenaInvitesMu.Unlock()

	return arenaTeamMutationResponse(pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_OK), nil
}

func (c *CharServer) AddArenaTeamMember(ctx context.Context, request *pb.AddArenaTeamMemberRequest) (*pb.ArenaTeamMutationResponse, error) {
	defer arenaTeamMutationLog("add member", request.GetRealmID(), request.GetArenaTeamID(), time.Now())

	if c.arenaTeams == nil {
		return arenaTeamMutationResponse(pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_FAILED), nil
	}

	if err := c.arenaTeams.AddMember(ctx, request.GetRealmID(), request.GetArenaTeamID(), request.GetPlayerGUID(), request.GetPersonalRating()); err != nil {
		return arenaTeamMutationError(err)
	}

	return arenaTeamMutationResponse(pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_OK), nil
}

func (c *CharServer) RemoveArenaTeamMember(ctx context.Context, request *pb.RemoveArenaTeamMemberRequest) (*pb.ArenaTeamMutationResponse, error) {
	defer arenaTeamMutationLog("remove member", request.GetRealmID(), request.GetArenaTeamID(), time.Now())

	if c.arenaTeams == nil {
		return arenaTeamMutationResponse(pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_FAILED), nil
	}

	team, err := c.arenaTeams.GetTeam(ctx, request.GetRealmID(), request.GetArenaTeamID())
	if err != nil {
		return arenaTeamMutationError(err)
	}

	if err := c.arenaTeams.RemoveMember(ctx, request.GetRealmID(), request.GetArenaTeamID(), request.GetPlayerGUID(), request.GetActorGUID()); err != nil {
		return arenaTeamMutationError(err)
	}

	if request.GetActorGUID() != 0 {
		targetName := arenaTeamMemberName(team, request.GetPlayerGUID())
		actorName := arenaTeamMemberName(team, request.GetActorGUID())
		if request.GetActorGUID() == request.GetPlayerGUID() {
			c.publishArenaTeamNativeEvent(
				request.GetRealmID(),
				team,
				arenaTeamNativeEventLeave,
				request.GetPlayerGUID(),
				targetName,
				team.Name,
			)
		} else {
			c.publishArenaTeamNativeEvent(
				request.GetRealmID(),
				team,
				arenaTeamNativeEventRemove,
				0,
				targetName,
				team.Name,
				actorName,
			)
		}
	}

	return arenaTeamMutationResponse(pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_OK), nil
}

func (c *CharServer) DisbandArenaTeam(ctx context.Context, request *pb.DisbandArenaTeamRequest) (*pb.ArenaTeamMutationResponse, error) {
	defer arenaTeamMutationLog("disband", request.GetRealmID(), request.GetArenaTeamID(), time.Now())

	if c.arenaTeams == nil {
		return arenaTeamMutationResponse(pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_FAILED), nil
	}

	team, err := c.arenaTeams.GetTeam(ctx, request.GetRealmID(), request.GetArenaTeamID())
	if err != nil {
		return arenaTeamMutationError(err)
	}

	if err := c.arenaTeams.Disband(ctx, request.GetRealmID(), request.GetArenaTeamID(), request.GetActorGUID()); err != nil {
		return arenaTeamMutationError(err)
	}

	if request.GetActorGUID() != 0 {
		c.publishArenaTeamNativeEvent(
			request.GetRealmID(),
			team,
			arenaTeamNativeEventDisbanded,
			0,
			arenaTeamMemberName(team, request.GetActorGUID()),
			team.Name,
		)
	}

	return arenaTeamMutationResponse(pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_OK), nil
}

func (c *CharServer) SetArenaTeamCaptain(ctx context.Context, request *pb.SetArenaTeamCaptainRequest) (*pb.ArenaTeamMutationResponse, error) {
	defer arenaTeamMutationLog("set captain", request.GetRealmID(), request.GetArenaTeamID(), time.Now())

	if c.arenaTeams == nil {
		return arenaTeamMutationResponse(pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_FAILED), nil
	}

	team, err := c.arenaTeams.GetTeam(ctx, request.GetRealmID(), request.GetArenaTeamID())
	if err != nil {
		return arenaTeamMutationError(err)
	}

	if err := c.arenaTeams.SetCaptain(ctx, request.GetRealmID(), request.GetArenaTeamID(), request.GetCaptainGUID(), request.GetActorGUID()); err != nil {
		return arenaTeamMutationError(err)
	}

	if request.GetActorGUID() != 0 {
		c.publishArenaTeamNativeEvent(
			request.GetRealmID(),
			team,
			arenaTeamNativeEventLeaderChanged,
			0,
			arenaTeamMemberName(team, request.GetActorGUID()),
			arenaTeamMemberName(team, request.GetCaptainGUID()),
			team.Name,
		)
	}

	return arenaTeamMutationResponse(pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_OK), nil
}

func (c *CharServer) SetArenaTeamName(ctx context.Context, request *pb.SetArenaTeamNameRequest) (*pb.ArenaTeamMutationResponse, error) {
	defer arenaTeamMutationLog("set name", request.GetRealmID(), request.GetArenaTeamID(), time.Now())

	if c.arenaTeams == nil {
		return arenaTeamMutationResponse(pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_FAILED), nil
	}

	if err := c.arenaTeams.SetName(ctx, request.GetRealmID(), request.GetArenaTeamID(), request.GetName(), request.GetActorGUID()); err != nil {
		return arenaTeamMutationError(err)
	}

	return arenaTeamMutationResponse(pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_OK), nil
}

func (c *CharServer) SaveArenaTeamStats(ctx context.Context, request *pb.SaveArenaTeamStatsRequest) (*pb.ArenaTeamMutationResponse, error) {
	defer arenaTeamMutationLog("save stats", request.GetRealmID(), request.GetArenaTeamID(), time.Now())

	if c.arenaTeams == nil {
		return arenaTeamMutationResponse(pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_FAILED), nil
	}

	members := make([]repo.ArenaTeamSaveStatsMember, 0, len(request.GetMembers()))
	for _, member := range request.GetMembers() {
		members = append(members, repo.ArenaTeamSaveStatsMember{
			PlayerGUID:       member.GetPlayerGUID(),
			PersonalRating:   member.GetPersonalRating(),
			WeekGames:        member.GetWeekGames(),
			WeekWins:         member.GetWeekWins(),
			SeasonGames:      member.GetSeasonGames(),
			SeasonWins:       member.GetSeasonWins(),
			MatchmakerRating: member.GetMatchmakerRating(),
			MaxMMR:           member.GetMaxMMR(),
			SaveArenaStats:   member.GetSaveArenaStats(),
		})
	}

	if err := c.arenaTeams.SaveStats(ctx, repo.ArenaTeamSaveStatsRequest{
		RealmID:     request.GetRealmID(),
		ArenaTeamID: request.GetArenaTeamID(),
		Rating:      request.GetRating(),
		WeekGames:   request.GetWeekGames(),
		WeekWins:    request.GetWeekWins(),
		SeasonGames: request.GetSeasonGames(),
		SeasonWins:  request.GetSeasonWins(),
		Rank:        request.GetRank(),
		Slot:        request.GetSlot(),
		Members:     members,
	}); err != nil {
		return arenaTeamMutationError(err)
	}

	return arenaTeamMutationResponse(pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_OK), nil
}

func (c *CharServer) DeleteAllArenaTeams(ctx context.Context, request *pb.DeleteAllArenaTeamsRequest) (*pb.ArenaTeamMutationResponse, error) {
	defer arenaTeamMutationLog("delete all", request.GetRealmID(), 0, time.Now())

	if c.arenaTeams == nil {
		return arenaTeamMutationResponse(pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_FAILED), nil
	}

	if err := c.arenaTeams.DeleteAll(ctx, request.GetRealmID()); err != nil {
		return arenaTeamMutationError(err)
	}

	return arenaTeamMutationResponse(pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_OK), nil
}

func (c *CharServer) ValidateArenaTeamCharacterDelete(ctx context.Context, request *pb.ValidateArenaTeamCharacterDeleteRequest) (*pb.ArenaTeamMutationResponse, error) {
	defer arenaTeamMutationLog("validate character delete", request.GetRealmID(), 0, time.Now())

	if c.arenaTeams == nil {
		return arenaTeamMutationResponse(pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_FAILED), nil
	}

	if err := c.arenaTeams.ValidateCharacterDelete(ctx, request.GetRealmID(), request.GetPlayerGUID()); err != nil {
		return arenaTeamMutationError(err)
	}

	return arenaTeamMutationResponse(pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_OK), nil
}

func (c *CharServer) RemovePlayerFromArenaTeams(ctx context.Context, request *pb.RemovePlayerFromArenaTeamsRequest) (*pb.ArenaTeamMutationResponse, error) {
	defer arenaTeamMutationLog("remove player from all teams", request.GetRealmID(), 0, time.Now())

	if c.arenaTeams == nil {
		return arenaTeamMutationResponse(pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_FAILED), nil
	}

	if err := c.arenaTeams.RemovePlayerFromTeams(ctx, request.GetRealmID(), request.GetPlayerGUID()); err != nil {
		return arenaTeamMutationError(err)
	}

	c.arenaInvitesMu.Lock()
	for key, invite := range c.arenaInvites {
		if key.realmID == request.GetRealmID() && (key.playerGUID == request.GetPlayerGUID() || invite.inviterGUID == request.GetPlayerGUID()) {
			delete(c.arenaInvites, key)
		}
	}
	c.arenaInvitesMu.Unlock()

	return arenaTeamMutationResponse(pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_OK), nil
}

func arenaTeamQueueStatusFromError(err error) (pb.ArenaTeamQueueDataForRatedArenaResponse_Status, bool) {
	switch {
	case errors.Is(err, repo.ErrArenaTeamNotFound):
		return pb.ArenaTeamQueueDataForRatedArenaResponse_NotFound, true
	case errors.Is(err, repo.ErrArenaTeamMemberMismatch):
		return pb.ArenaTeamQueueDataForRatedArenaResponse_MemberMismatch, true
	case errors.Is(err, repo.ErrArenaTeamPartySize):
		return pb.ArenaTeamQueueDataForRatedArenaResponse_InvalidPartySize, true
	case errors.Is(err, repo.ErrArenaTeamInvalidType):
		return pb.ArenaTeamQueueDataForRatedArenaResponse_InvalidType, true
	default:
		return pb.ArenaTeamQueueDataForRatedArenaResponse_Failed, false
	}
}

func arenaTeamMutationResponse(status pb.ArenaTeamMutationStatus) *pb.ArenaTeamMutationResponse {
	return &pb.ArenaTeamMutationResponse{
		Api:    ver,
		Status: status,
	}
}

func arenaTeamMutationError(err error) (*pb.ArenaTeamMutationResponse, error) {
	status, handled := arenaTeamMutationStatusFromError(err)
	if handled {
		return arenaTeamMutationResponse(status), nil
	}
	return nil, err
}

func arenaTeamMutationStatusFromError(err error) (pb.ArenaTeamMutationStatus, bool) {
	switch {
	case errors.Is(err, repo.ErrArenaTeamNotFound):
		return pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_NOT_FOUND, true
	case errors.Is(err, repo.ErrArenaTeamMemberMismatch):
		return pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_MEMBER_MISMATCH, true
	case errors.Is(err, repo.ErrArenaTeamInvalidType):
		return pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_INVALID_TYPE, true
	case errors.Is(err, repo.ErrArenaTeamNameExists):
		return pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_NAME_EXISTS, true
	case errors.Is(err, repo.ErrArenaTeamAlreadyInTeam):
		return pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_ALREADY_IN_TEAM, true
	case errors.Is(err, repo.ErrArenaTeamRosterFull):
		return pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_ROSTER_FULL, true
	case errors.Is(err, repo.ErrArenaTeamInvalidName):
		return pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_INVALID_NAME, true
	case errors.Is(err, repo.ErrArenaTeamPermission):
		return pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_PERMISSION_DENIED, true
	case errors.Is(err, repo.ErrArenaTeamLeaderLeave):
		return pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_LEADER_LEAVE, true
	default:
		return pb.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_FAILED, false
	}
}

func (c *CharServer) arenaTeamData(ctx context.Context, realmID uint32, team *repo.ArenaTeamDetails) *pb.ArenaTeamData {
	if team == nil {
		return nil
	}

	online := map[uint64]struct{}{}
	if c.onlineChars != nil && len(team.Members) > 0 {
		guids := make([]uint64, 0, len(team.Members))
		for _, member := range team.Members {
			guids = append(guids, member.PlayerGUID)
		}
		chars, err := c.onlineChars.CharactersByRealmAndGUIDs(ctx, realmID, guids)
		if err == nil {
			for _, char := range chars {
				online[char.CharGUID] = struct{}{}
			}
		}
	}

	members := make([]*pb.ArenaTeamMemberData, 0, len(team.Members))
	for _, member := range team.Members {
		_, isOnline := online[member.PlayerGUID]
		members = append(members, &pb.ArenaTeamMemberData{
			PlayerGUID:       member.PlayerGUID,
			Name:             member.Name,
			Online:           isOnline,
			Level:            uint32(member.Level),
			Class:            uint32(member.Class),
			WeekGames:        member.WeekGames,
			WeekWins:         member.WeekWins,
			SeasonGames:      member.SeasonGames,
			SeasonWins:       member.SeasonWins,
			PersonalRating:   member.PersonalRating,
			MatchmakerRating: member.MatchmakerRating,
			MaxMMR:           member.MaxMMR,
		})
	}

	return &pb.ArenaTeamData{
		ArenaTeamID:     team.ArenaTeamID,
		Name:            team.Name,
		CaptainGUID:     team.CaptainGUID,
		Type:            uint32(team.Type),
		Rating:          team.Rating,
		WeekGames:       team.WeekGames,
		WeekWins:        team.WeekWins,
		SeasonGames:     team.SeasonGames,
		SeasonWins:      team.SeasonWins,
		Rank:            team.Rank,
		BackgroundColor: team.BackgroundColor,
		EmblemStyle:     uint32(team.EmblemStyle),
		EmblemColor:     team.EmblemColor,
		BorderStyle:     uint32(team.BorderStyle),
		BorderColor:     team.BorderColor,
		Members:         members,
	}
}

func arenaTeamHasMemberData(team *repo.ArenaTeamDetails, playerGUID uint64) bool {
	if team == nil {
		return false
	}
	for _, member := range team.Members {
		if member.PlayerGUID == playerGUID {
			return true
		}
	}
	return false
}

func arenaTeamMemberName(team *repo.ArenaTeamDetails, playerGUID uint64) string {
	if team == nil {
		return ""
	}
	for _, member := range team.Members {
		if member.PlayerGUID == playerGUID {
			return member.Name
		}
	}
	return ""
}

func (c *CharServer) arenaTeamTargetIgnoresInviter(ctx context.Context, realmID uint32, targetGUID uint64, inviterGUID uint64) (bool, error) {
	if c.friendsService == nil {
		return false, nil
	}

	friends, err := c.friendsService.GetFriendsList(ctx, realmID, targetGUID)
	if err != nil {
		return false, err
	}
	if friends == nil {
		return false, nil
	}
	for _, ignoredGUID := range friends.Ignored {
		if ignoredGUID == inviterGUID {
			return true, nil
		}
	}
	return false, nil
}

func (c *CharServer) publishArenaTeamNativeEvent(realmID uint32, team *repo.ArenaTeamDetails, event uint8, eventGUID uint64, args ...string) {
	if c.eventsProducer == nil || team == nil || len(team.Members) == 0 {
		return
	}

	receiverGUIDs := make([]uint64, 0, len(team.Members))
	for _, member := range team.Members {
		receiverGUIDs = append(receiverGUIDs, member.PlayerGUID)
	}

	if err := c.eventsProducer.ArenaTeamNativeEvent(&events.CharEventArenaTeamNativeEventPayload{
		RealmID:       realmID,
		ReceiverGUIDs: receiverGUIDs,
		ArenaTeamID:   team.ArenaTeamID,
		Event:         event,
		EventGUID:     eventGUID,
		Args:          args,
	}); err != nil {
		log.Error().
			Err(err).
			Uint32("realmID", realmID).
			Uint32("arenaTeamID", team.ArenaTeamID).
			Uint8("event", event).
			Msg("Failed to publish arena team native event")
	}
}

func arenaTeamMutationLog(action string, realmID uint32, arenaTeamID uint32, start time.Time) {
	log.Debug().
		Str("action", action).
		Uint32("realmID", realmID).
		Uint32("arenaTeamID", arenaTeamID).
		Str("timeTook", time.Since(start).String()).
		Msg("Handled arena team mutation request")
}

func arenaTeamCreateStatusFromError(err error) (pb.CreateArenaTeamFromPetitionResponse_Status, bool) {
	switch {
	case errors.Is(err, repo.ErrArenaTeamNotFound):
		return pb.CreateArenaTeamFromPetitionResponse_NotFound, true
	case errors.Is(err, repo.ErrArenaTeamNotOwner):
		return pb.CreateArenaTeamFromPetitionResponse_NotOwner, true
	case errors.Is(err, repo.ErrArenaTeamInvalidType):
		return pb.CreateArenaTeamFromPetitionResponse_InvalidType, true
	case errors.Is(err, repo.ErrArenaTeamNameExists):
		return pb.CreateArenaTeamFromPetitionResponse_NameExists, true
	case errors.Is(err, repo.ErrArenaTeamAlreadyInTeam):
		return pb.CreateArenaTeamFromPetitionResponse_AlreadyInTeam, true
	case errors.Is(err, repo.ErrArenaTeamNotEnoughSigns):
		return pb.CreateArenaTeamFromPetitionResponse_NotEnoughSignatures, true
	case errors.Is(err, repo.ErrArenaTeamInvalidName):
		return pb.CreateArenaTeamFromPetitionResponse_InvalidName, true
	default:
		return pb.CreateArenaTeamFromPetitionResponse_Failed, false
	}
}
