package server

import (
	"context"

	"github.com/walkline/ToCloud9/apps/guildserver"
	"github.com/walkline/ToCloud9/apps/guildserver/service"
	"github.com/walkline/ToCloud9/gen/guilds/pb"
)

// GuildServer is guild server that handles grpc requests.
type GuildServer struct {
	guildsService service.GuildService
}

// NewGuildServer creates new guild server.
func NewGuildServer(guildsService service.GuildService) pb.GuildServiceServer {
	return &GuildServer{
		guildsService: guildsService,
	}
}

// GetGuildInfo handles guild query request for game client.
func (g *GuildServer) GetGuildInfo(ctx context.Context, params *pb.GetInfoParams) (*pb.GetInfoResponse, error) {
	guild, err := g.guildsService.GuildByRealmAndID(ctx, params.RealmID, params.GuildID)
	if err != nil {
		return nil, err
	}

	rankNames := make([]string, len(guild.GuildRanks))
	for i := range guild.GuildRanks {
		rankNames[i] = guild.GuildRanks[i].Name
	}

	return &pb.GetInfoResponse{
		Api:             guildserver.Ver,
		GuildID:         guild.ID,
		GuildName:       guild.Name,
		EmblemStyle:     uint32(guild.Emblem.Style),
		EmblemColor:     uint32(guild.Emblem.Color),
		BorderStyle:     uint32(guild.Emblem.BorderStyle),
		BorderColor:     uint32(guild.Emblem.BorderColor),
		BackgroundColor: uint32(guild.Emblem.BackgroundColor),
		RankNames:       rankNames,
	}, nil
}

// GetRosterInfo handles Roster Info request for game client.
func (g *GuildServer) GetRosterInfo(ctx context.Context, params *pb.GetRosterInfoParams) (*pb.GetRosterInfoResponse, error) {
	guild, err := g.guildsService.GuildByRealmAndID(ctx, params.RealmID, params.GuildID)
	if err != nil {
		return nil, err
	}

	members := make([]*pb.GetRosterInfoResponse_Member, len(guild.GuildMembers))
	for i := range guild.GuildMembers {
		members[i] = &pb.GetRosterInfoResponse_Member{
			Guid:        guild.GuildMembers[i].PlayerGUID,
			Name:        guild.GuildMembers[i].Name,
			Status:      uint32(guild.GuildMembers[i].Status),
			RankID:      uint32(guild.GuildMembers[i].Rank),
			Lvl:         uint32(guild.GuildMembers[i].Lvl),
			ClassID:     uint32(guild.GuildMembers[i].Class),
			Gender:      uint32(guild.GuildMembers[i].Gender),
			AreaID:      guild.GuildMembers[i].AreaID,
			LogoutTime:  guild.GuildMembers[i].LogoutTime,
			Note:        guild.GuildMembers[i].PublicNote,
			OfficerNote: guild.GuildMembers[i].OfficerNote,
		}
	}

	ranks := make([]*pb.GetRosterInfoResponse_Rank, len(guild.GuildRanks))
	for i := range guild.GuildRanks {
		ranks[i] = &pb.GetRosterInfoResponse_Rank{
			Id:        uint32(guild.GuildRanks[i].Rank),
			Flags:     guild.GuildRanks[i].Rights,
			GoldLimit: guild.GuildRanks[i].MoneyPerDay,
		}
	}

	return &pb.GetRosterInfoResponse{
		Api: guildserver.Ver,
		Guild: &pb.GetRosterInfoResponse_Guild{
			Id:          guild.ID,
			WelcomeText: guild.MessageOfTheDay,
			InfoText:    guild.Info,
			Members:     members,
			Ranks:       ranks,
		},
	}, nil
}

// InviteMember handles members invite.
func (g *GuildServer) InviteMember(ctx context.Context, params *pb.InviteMemberParams) (*pb.InviteMemberResponse, error) {
	err := g.guildsService.InviteMember(ctx, params.RealmID, params.Inviter, params.Invitee, params.InviteeName)
	if err != nil {
		return nil, err
	}

	return &pb.InviteMemberResponse{
		Api: guildserver.Ver,
	}, nil
}

// InviteAccepted handles accept of guild invite.
func (g *GuildServer) InviteAccepted(ctx context.Context, params *pb.InviteAcceptedParams) (*pb.InviteAcceptedResponse, error) {
	guildID, err := g.guildsService.InviteAccepted(ctx, params.RealmID, service.InviteAcceptedParams{
		CharGUID:    params.Character.Guid,
		CharName:    params.Character.Name,
		CharRace:    uint8(params.Character.Race),
		CharClass:   uint8(params.Character.ClassID),
		CharLvl:     uint8(params.Character.Lvl),
		CharGender:  uint8(params.Character.Gender),
		CharAreaID:  params.Character.AreaID,
		CharAccount: params.Character.AccountID,
	})
	if err != nil {
		return nil, err
	}

	return &pb.InviteAcceptedResponse{
		Api:     guildserver.Ver,
		GuildID: guildID,
	}, nil
}

// Leave handles players leave from the guild.
func (g *GuildServer) Leave(ctx context.Context, params *pb.LeaveParams) (*pb.LeaveResponse, error) {
	err := g.guildsService.Leave(ctx, params.RealmID, params.Leaver)
	if err != nil {
		return nil, err
	}
	return &pb.LeaveResponse{
		Api: guildserver.Ver,
	}, nil
}

// Kick handles kick of th guild member.
func (g *GuildServer) Kick(ctx context.Context, params *pb.KickParams) (*pb.KickResponse, error) {
	err := g.guildsService.Kick(ctx, params.RealmID, params.Kicker, params.Target)
	if err != nil {
		return nil, err
	}
	return &pb.KickResponse{
		Api: guildserver.Ver,
	}, nil
}

// SetMessageOfTheDay sets the message of the day for the guild.
func (g *GuildServer) SetMessageOfTheDay(ctx context.Context, params *pb.SetMessageOfTheDayParams) (*pb.SetMessageOfTheDayResponse, error) {
	err := g.guildsService.SetMessageOfTheDay(ctx, params.RealmID, params.ChangerGUID, params.MessageOfTheDay)
	if err != nil {
		return nil, err
	}
	return &pb.SetMessageOfTheDayResponse{
		Api: guildserver.Ver,
	}, nil
}

// SetMemberPublicNote sets public note for the guild member.
func (g *GuildServer) SetMemberPublicNote(ctx context.Context, params *pb.SetNoteParams) (*pb.SetNoteResponse, error) {
	err := g.guildsService.SetMemberPublicNote(ctx, params.RealmID, params.ChangerGUID, params.TargetGUID, params.Note)
	if err != nil {
		return nil, err
	}
	return &pb.SetNoteResponse{
		Api: guildserver.Ver,
	}, nil
}

// SetMemberOfficerNote sets officer note for the guild member.
func (g *GuildServer) SetMemberOfficerNote(ctx context.Context, params *pb.SetNoteParams) (*pb.SetNoteResponse, error) {
	err := g.guildsService.SetMemberOfficerNote(ctx, params.RealmID, params.ChangerGUID, params.TargetGUID, params.Note)
	if err != nil {
		return nil, err
	}
	return &pb.SetNoteResponse{
		Api: guildserver.Ver,
	}, nil
}

// SetGuildInfo sets info text for the guild.
func (g *GuildServer) SetGuildInfo(ctx context.Context, params *pb.SetGuildInfoParams) (*pb.SetGuildInfoResponse, error) {
	err := g.guildsService.SetGuildInfo(ctx, params.RealmID, params.ChangerGUID, params.Info)
	if err != nil {
		return nil, err
	}
	return &pb.SetGuildInfoResponse{
		Api: guildserver.Ver,
	}, nil
}

// UpdateRank handles guild rank update.
func (g *GuildServer) UpdateRank(ctx context.Context, params *pb.RankUpdateParams) (*pb.RankUpdateResponse, error) {
	err := g.guildsService.UpdateGuildRank(ctx, params.RealmID, params.ChangerGUID, service.GuildRank{
		RankID:      uint8(params.Rank),
		Name:        params.RankName,
		Rights:      params.Rights,
		MoneyPerDay: params.MoneyPerDay,
	})
	if err != nil {
		return nil, err
	}
	return &pb.RankUpdateResponse{
		Api: guildserver.Ver,
	}, nil
}

// AddRank handles adding new rank for guild.
func (g *GuildServer) AddRank(ctx context.Context, params *pb.AddRankParams) (*pb.AddRankResponse, error) {
	err := g.guildsService.AddGuildRank(ctx, params.RealmID, params.ChangerGUID, params.RankName)
	if err != nil {
		return nil, err
	}
	return &pb.AddRankResponse{
		Api: guildserver.Ver,
	}, nil
}

// DeleteLastRank handles deletion of the last rank for guild.
func (g *GuildServer) DeleteLastRank(ctx context.Context, params *pb.DeleteLastRankParams) (*pb.DeleteLastRankResponse, error) {
	err := g.guildsService.DeleteLastGuildRank(ctx, params.RealmID, params.ChangerGUID)
	if err != nil {
		return nil, err
	}
	return &pb.DeleteLastRankResponse{
		Api: guildserver.Ver,
	}, nil
}

// PromoteMember handles promotion of guild member.
func (g *GuildServer) PromoteMember(ctx context.Context, params *pb.PromoteDemoteParams) (*pb.PromoteDemoteResponse, error) {
	err := g.guildsService.PromoteMember(ctx, params.RealmID, params.ChangerGUID, params.TargetGUID)
	if err != nil {
		return nil, err
	}
	return &pb.PromoteDemoteResponse{
		Api: guildserver.Ver,
	}, nil
}

// DemoteMember handles demotion of guild member.
func (g *GuildServer) DemoteMember(ctx context.Context, params *pb.PromoteDemoteParams) (*pb.PromoteDemoteResponse, error) {
	err := g.guildsService.DemoteMember(ctx, params.RealmID, params.ChangerGUID, params.TargetGUID)
	if err != nil {
		return nil, err
	}
	return &pb.PromoteDemoteResponse{
		Api: guildserver.Ver,
	}, nil
}

// SendGuildMessage sends new message to the guild members.
func (g *GuildServer) SendGuildMessage(ctx context.Context, params *pb.SendGuildMessageParams) (*pb.SendGuildMessageResponse, error) {
	err := g.guildsService.SendGuildMessage(ctx, params.RealmID, params.SenderGUID, params.Message, params.Language, params.IsOfficerMessage)
	if err != nil {
		return nil, err
	}

	return &pb.SendGuildMessageResponse{
		Api: guildserver.Ver,
	}, nil
}
