package server

import (
	"context"

	"github.com/walkline/ToCloud9/apps/guildserver"
	"github.com/walkline/ToCloud9/apps/guildserver/repo"
	"github.com/walkline/ToCloud9/apps/guildserver/service"
	"github.com/walkline/ToCloud9/gen/guilds/pb"
)

// GuildServer is guild server that handles grpc requests.
type GuildServer struct {
	pb.UnimplementedGuildServiceServer
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
			BankWithdraw: []uint32{
				guild.GuildMembers[i].BankWithdraw[0],
				guild.GuildMembers[i].BankWithdraw[1],
				guild.GuildMembers[i].BankWithdraw[2],
				guild.GuildMembers[i].BankWithdraw[3],
				guild.GuildMembers[i].BankWithdraw[4],
				guild.GuildMembers[i].BankWithdraw[5],
				guild.GuildMembers[i].BankWithdraw[6],
			},
		}
	}

	ranks := make([]*pb.GetRosterInfoResponse_Rank, len(guild.GuildRanks))
	for i := range guild.GuildRanks {
		bankTabRights := make([]*pb.GetRosterInfoResponse_Rank_BankTabRight, len(guild.GuildRanks[i].BankTabRights))
		for tabID := range guild.GuildRanks[i].BankTabRights {
			right := guild.GuildRanks[i].BankTabRights[tabID]
			bankTabRights[tabID] = &pb.GetRosterInfoResponse_Rank_BankTabRight{
				TabID:             uint32(right.TabID),
				Flags:             right.Flags,
				WithdrawItemLimit: right.WithdrawItemLimit,
			}
		}

		ranks[i] = &pb.GetRosterInfoResponse_Rank{
			Id:            uint32(guild.GuildRanks[i].Rank),
			Flags:         guild.GuildRanks[i].Rights,
			GoldLimit:     guild.GuildRanks[i].MoneyPerDay,
			BankTabRights: bankTabRights,
		}
	}

	return &pb.GetRosterInfoResponse{
		Api: guildserver.Ver,
		Guild: &pb.GetRosterInfoResponse_Guild{
			Id:                guild.ID,
			WelcomeText:       guild.MessageOfTheDay,
			InfoText:          guild.Info,
			Members:           members,
			Ranks:             ranks,
			PurchasedBankTabs: uint32(guild.PurchasedBankTabs),
		},
	}, nil
}

// InviteMember handles members invite.
func (g *GuildServer) InviteMember(ctx context.Context, params *pb.InviteMemberParams) (*pb.InviteMemberResponse, error) {
	err := g.guildsService.InviteMember(ctx, params.RealmID, params.Inviter, params.Invitee, params.InviteeName, uint8(params.InviteeRace), params.AllowCrossFaction)
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

		AllowCrossFaction: params.AllowCrossFaction,
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
	var bankTabRights [repo.GuildBankMaxTabs]repo.GuildBankTabRight
	for _, right := range params.BankTabRights {
		if right.TabID < repo.GuildBankMaxTabs {
			bankTabRights[right.TabID] = repo.GuildBankTabRight{
				TabID:             uint8(right.TabID),
				Flags:             right.Flags,
				WithdrawItemLimit: right.WithdrawItemLimit,
			}
		}
	}

	err := g.guildsService.UpdateGuildRank(ctx, params.RealmID, params.ChangerGUID, service.GuildRank{
		RankID:        uint8(params.Rank),
		Name:          params.RankName,
		Rights:        params.Rights,
		MoneyPerDay:   params.MoneyPerDay,
		BankTabRights: bankTabRights,
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
	err := g.guildsService.SendGuildMessage(ctx, params.RealmID, params.SenderGUID, params.Message, params.Language, params.IsOfficerMessage, uint8(params.SenderChatTag))
	if err != nil {
		return nil, err
	}

	return &pb.SendGuildMessageResponse{
		Api: guildserver.Ver,
	}, nil
}

// GetGuildPetition handles a native guild petition lookup.
func (g *GuildServer) GetGuildPetition(ctx context.Context, params *pb.GetGuildPetitionParams) (*pb.GetGuildPetitionResponse, error) {
	petition, err := g.guildsService.GuildPetitionByGUID(ctx, params.RealmID, params.PetitionGUID)
	if err != nil {
		return nil, err
	}

	return &pb.GetGuildPetitionResponse{
		Api:      guildserver.Ver,
		Petition: repoGuildPetitionToPB(petition),
	}, nil
}

// OfferGuildPetition handles same-realm distributed guild charter offers.
func (g *GuildServer) OfferGuildPetition(ctx context.Context, params *pb.OfferGuildPetitionParams) (*pb.OfferGuildPetitionResponse, error) {
	status, err := g.guildsService.OfferGuildPetition(ctx, params.RealmID, params.OwnerGUID, params.TargetGUID, params.TargetName, params.PetitionGUID)
	if err != nil {
		return nil, err
	}

	return &pb.OfferGuildPetitionResponse{
		Api:    guildserver.Ver,
		Status: pb.OfferGuildPetitionResponse_Status(status),
	}, nil
}

// SignGuildPetition handles same-realm distributed guild charter signatures.
func (g *GuildServer) SignGuildPetition(ctx context.Context, params *pb.SignGuildPetitionParams) (*pb.SignGuildPetitionResponse, error) {
	status, err := g.guildsService.SignGuildPetition(ctx, params.RealmID, params.SignerGUID, params.SignerName, params.SignerAccountID, params.SignerGuildID, params.PetitionGUID)
	if err != nil {
		return nil, err
	}

	return &pb.SignGuildPetitionResponse{
		Api:    guildserver.Ver,
		Status: pb.SignGuildPetitionResponse_Status(status),
	}, nil
}

func (g *GuildServer) GetGuildBank(ctx context.Context, params *pb.GetGuildBankParams) (*pb.GetGuildBankResponse, error) {
	bank, remaining, status, err := g.guildsService.GetGuildBank(ctx, params.RealmID, params.GuildID, params.MemberGUID, uint8(params.TabID), params.FullUpdate)
	if err != nil {
		return nil, err
	}
	resp := &pb.GetGuildBankResponse{
		Api:                  guildserver.Ver,
		Status:               guildBankStatusToPB(status),
		TabID:                params.TabID,
		WithdrawalsRemaining: remaining,
		FullUpdate:           params.FullUpdate,
	}
	if bank != nil {
		resp.Money = bank.Money
		resp.Tabs = repoGuildBankTabsToPB(bank.Tabs)
		for _, tab := range bank.Tabs {
			if tab.TabID == uint8(params.TabID) {
				resp.Items = repoGuildBankItemsToPB(tab.Items)
				break
			}
		}
	}
	return resp, nil
}

func (g *GuildServer) GetGuildBankLog(ctx context.Context, params *pb.GetGuildBankLogParams) (*pb.GetGuildBankLogResponse, error) {
	entries, status, err := g.guildsService.GetGuildBankLog(ctx, params.RealmID, params.GuildID, params.MemberGUID, uint8(params.TabID))
	if err != nil {
		return nil, err
	}
	return &pb.GetGuildBankLogResponse{
		Api:     guildserver.Ver,
		Status:  guildBankStatusToPB(status),
		TabID:   params.TabID,
		Entries: repoGuildBankLogEntriesToPB(entries),
	}, nil
}

func (g *GuildServer) GetGuildBankTabText(ctx context.Context, params *pb.GetGuildBankTabTextParams) (*pb.GetGuildBankTabTextResponse, error) {
	text, status, err := g.guildsService.GetGuildBankTabText(ctx, params.RealmID, params.GuildID, params.MemberGUID, uint8(params.TabID))
	if err != nil {
		return nil, err
	}
	return &pb.GetGuildBankTabTextResponse{
		Api:    guildserver.Ver,
		Status: guildBankStatusToPB(status),
		TabID:  params.TabID,
		Text:   text,
	}, nil
}

func (g *GuildServer) UpdateGuildBankTab(ctx context.Context, params *pb.UpdateGuildBankTabParams) (*pb.GuildBankActionResponse, error) {
	status, err := g.guildsService.UpdateGuildBankTab(ctx, params.RealmID, params.GuildID, params.MemberGUID, uint8(params.TabID), params.Name, params.Icon)
	if err != nil {
		return nil, err
	}
	return &pb.GuildBankActionResponse{Api: guildserver.Ver, Status: guildBankStatusToPB(status)}, nil
}

func (g *GuildServer) SetGuildBankTabText(ctx context.Context, params *pb.SetGuildBankTabTextParams) (*pb.GuildBankActionResponse, error) {
	status, err := g.guildsService.SetGuildBankTabText(ctx, params.RealmID, params.GuildID, params.MemberGUID, uint8(params.TabID), params.Text)
	if err != nil {
		return nil, err
	}
	return &pb.GuildBankActionResponse{Api: guildserver.Ver, Status: guildBankStatusToPB(status)}, nil
}

func (g *GuildServer) BuyGuildBankTab(ctx context.Context, params *pb.BuyGuildBankTabParams) (*pb.BuyGuildBankTabResponse, error) {
	status, err := g.guildsService.BuyGuildBankTab(ctx, params.RealmID, params.GuildID, params.MemberGUID, uint8(params.TabID), 0)
	if err != nil {
		return nil, err
	}
	return &pb.BuyGuildBankTabResponse{Api: guildserver.Ver, Status: guildBankStatusToPB(status)}, nil
}

func (g *GuildServer) DepositGuildBankMoney(ctx context.Context, params *pb.DepositGuildBankMoneyParams) (*pb.GuildBankActionResponse, error) {
	status, err := g.guildsService.DepositGuildBankMoney(ctx, params.RealmID, params.GuildID, params.MemberGUID, params.Amount)
	if err != nil {
		return nil, err
	}
	return &pb.GuildBankActionResponse{Api: guildserver.Ver, Status: guildBankStatusToPB(status)}, nil
}

func (g *GuildServer) WithdrawGuildBankMoney(ctx context.Context, params *pb.WithdrawGuildBankMoneyParams) (*pb.GuildBankActionResponse, error) {
	logGUID, status, err := g.guildsService.WithdrawGuildBankMoney(ctx, params.RealmID, params.GuildID, params.MemberGUID, params.Amount, params.Repair)
	if err != nil {
		return nil, err
	}
	return &pb.GuildBankActionResponse{Api: guildserver.Ver, Status: guildBankStatusToPB(status), LogGUID: logGUID}, nil
}

func (g *GuildServer) RollbackGuildBankMoneyWithdraw(ctx context.Context, params *pb.RollbackGuildBankMoneyWithdrawParams) (*pb.GuildBankActionResponse, error) {
	status, err := g.guildsService.RollbackGuildBankMoneyWithdraw(ctx, params.RealmID, params.GuildID, params.MemberGUID, params.Amount, params.Repair, params.LogGUID)
	if err != nil {
		return nil, err
	}
	return &pb.GuildBankActionResponse{Api: guildserver.Ver, Status: guildBankStatusToPB(status)}, nil
}

func (g *GuildServer) DepositGuildBankItem(ctx context.Context, params *pb.DepositGuildBankItemParams) (*pb.GuildBankItemMutationResponse, error) {
	status, err := g.guildsService.DepositGuildBankItem(ctx, params.RealmID, params.GuildID, params.MemberGUID, uint8(params.TabID), uint8(params.SlotID), pbGuildBankItemToRepo(params.Item))
	if err != nil {
		return nil, err
	}
	return &pb.GuildBankItemMutationResponse{Api: guildserver.Ver, Status: guildBankStatusToPB(status), ChangedSlots: []uint32{params.SlotID}}, nil
}

func (g *GuildServer) WithdrawGuildBankItem(ctx context.Context, params *pb.WithdrawGuildBankItemParams) (*pb.GuildBankItemMutationResponse, error) {
	item, logGUID, status, err := g.guildsService.WithdrawGuildBankItem(ctx, params.RealmID, params.GuildID, params.MemberGUID, uint8(params.TabID), uint8(params.SlotID), params.Count)
	if err != nil {
		return nil, err
	}
	return &pb.GuildBankItemMutationResponse{Api: guildserver.Ver, Status: guildBankStatusToPB(status), Item: repoGuildBankItemToPB(item), ChangedSlots: []uint32{params.SlotID}, LogGUID: logGUID}, nil
}

func (g *GuildServer) RollbackGuildBankItemWithdraw(ctx context.Context, params *pb.RollbackGuildBankItemWithdrawParams) (*pb.GuildBankItemMutationResponse, error) {
	changed, status, err := g.guildsService.RollbackGuildBankItemWithdraw(ctx, params.RealmID, params.GuildID, params.MemberGUID, uint8(params.TabID), uint8(params.SlotID), pbGuildBankItemToRepo(params.Item), params.LogGUID)
	if err != nil {
		return nil, err
	}
	changedSlots := make([]uint32, len(changed))
	for i := range changed {
		changedSlots[i] = uint32(changed[i])
	}
	return &pb.GuildBankItemMutationResponse{Api: guildserver.Ver, Status: guildBankStatusToPB(status), ChangedSlots: changedSlots}, nil
}

func (g *GuildServer) MoveGuildBankItem(ctx context.Context, params *pb.MoveGuildBankItemParams) (*pb.GuildBankItemMutationResponse, error) {
	changed, status, err := g.guildsService.MoveGuildBankItem(ctx, params.RealmID, params.GuildID, params.MemberGUID, uint8(params.SourceTabID), uint8(params.SourceSlotID), uint8(params.DestinationTabID), uint8(params.DestinationSlotID), params.Count)
	if err != nil {
		return nil, err
	}
	changedSlots := make([]uint32, len(changed))
	for i := range changed {
		changedSlots[i] = uint32(changed[i])
	}
	return &pb.GuildBankItemMutationResponse{Api: guildserver.Ver, Status: guildBankStatusToPB(status), ChangedSlots: changedSlots}, nil
}

func repoGuildPetitionToPB(petition *repo.GuildPetition) *pb.GuildPetition {
	if petition == nil {
		return nil
	}

	signatures := make([]*pb.GuildPetitionSignature, len(petition.Signatures))
	for i := range petition.Signatures {
		signatures[i] = &pb.GuildPetitionSignature{
			PlayerGUID:    petition.Signatures[i].PlayerGUID,
			PlayerAccount: petition.Signatures[i].PlayerAccount,
		}
	}

	return &pb.GuildPetition{
		PetitionGUID: petition.PetitionGUID,
		PetitionID:   petition.PetitionID,
		OwnerGUID:    petition.OwnerGUID,
		Name:         petition.Name,
		Type:         uint32(petition.Type),
		Signatures:   signatures,
	}
}

func guildBankStatusToPB(status service.GuildBankStatus) pb.GuildBankStatus_Status {
	switch status {
	case service.GuildBankStatusOK:
		return pb.GuildBankStatus_Ok
	case service.GuildBankStatusGuildNotFound:
		return pb.GuildBankStatus_GuildNotFound
	case service.GuildBankStatusNotInGuild:
		return pb.GuildBankStatus_NotInGuild
	case service.GuildBankStatusNotEnoughRights:
		return pb.GuildBankStatus_NotEnoughRights
	case service.GuildBankStatusInvalidTab:
		return pb.GuildBankStatus_InvalidTab
	case service.GuildBankStatusInvalidSlot:
		return pb.GuildBankStatus_InvalidSlot
	case service.GuildBankStatusNotEnoughMoney:
		return pb.GuildBankStatus_NotEnoughMoney
	case service.GuildBankStatusBankFull:
		return pb.GuildBankStatus_BankFull
	case service.GuildBankStatusWithdrawLimit:
		return pb.GuildBankStatus_WithdrawLimit
	case service.GuildBankStatusItemNotFound:
		return pb.GuildBankStatus_ItemNotFound
	default:
		return pb.GuildBankStatus_Failed
	}
}

func repoGuildBankTabsToPB(tabs []repo.GuildBankTab) []*pb.GuildBankTab {
	result := make([]*pb.GuildBankTab, len(tabs))
	for i := range tabs {
		result[i] = &pb.GuildBankTab{
			TabID: uint32(tabs[i].TabID),
			Name:  tabs[i].Name,
			Icon:  tabs[i].Icon,
			Text:  tabs[i].Text,
			Items: repoGuildBankItemsToPB(tabs[i].Items),
		}
	}
	return result
}

func repoGuildBankItemsToPB(items []repo.GuildBankItem) []*pb.GuildBankItem {
	result := make([]*pb.GuildBankItem, len(items))
	for i := range items {
		result[i] = repoGuildBankItemToPB(&items[i])
	}
	return result
}

func repoGuildBankItemToPB(item *repo.GuildBankItem) *pb.GuildBankItem {
	if item == nil {
		return nil
	}
	sockets := make([]*pb.GuildBankSocketEnchant, len(item.SocketEnchants))
	for i := range item.SocketEnchants {
		sockets[i] = &pb.GuildBankSocketEnchant{
			SocketIndex:     uint32(item.SocketEnchants[i].SocketIndex),
			SocketEnchantID: item.SocketEnchants[i].SocketEnchantID,
		}
	}
	return &pb.GuildBankItem{
		ItemGUID:           item.ItemGUID,
		Entry:              item.Entry,
		Slot:               uint32(item.Slot),
		Count:              item.Count,
		Flags:              item.Flags,
		RandomPropertyID:   item.RandomPropertyID,
		RandomPropertySeed: item.RandomPropertySeed,
		Durability:         item.Durability,
		EnchantmentID:      item.EnchantmentID,
		SocketEnchants:     sockets,
		Charges:            item.Charges,
		Text:               item.Text,
	}
}

func pbGuildBankItemToRepo(item *pb.GuildBankItem) repo.GuildBankItem {
	if item == nil {
		return repo.GuildBankItem{}
	}
	sockets := make([]repo.GuildBankSocketEnchant, len(item.SocketEnchants))
	for i := range item.SocketEnchants {
		sockets[i] = repo.GuildBankSocketEnchant{
			SocketIndex:     uint8(item.SocketEnchants[i].SocketIndex),
			SocketEnchantID: item.SocketEnchants[i].SocketEnchantID,
		}
	}
	return repo.GuildBankItem{
		ItemGUID:           item.ItemGUID,
		Entry:              item.Entry,
		Slot:               uint8(item.Slot),
		Count:              item.Count,
		Flags:              item.Flags,
		RandomPropertyID:   item.RandomPropertyID,
		RandomPropertySeed: item.RandomPropertySeed,
		Durability:         item.Durability,
		EnchantmentID:      item.EnchantmentID,
		SocketEnchants:     sockets,
		Charges:            item.Charges,
		Text:               item.Text,
	}
}

func repoGuildBankLogEntriesToPB(entries []repo.GuildBankLogEntry) []*pb.GuildBankLogEntry {
	result := make([]*pb.GuildBankLogEntry, len(entries))
	for i := range entries {
		result[i] = &pb.GuildBankLogEntry{
			PlayerGUID: entries[i].PlayerGUID,
			TimeOffset: entries[i].TimeOffset,
			EntryType:  int32(entries[i].EntryType),
			Money:      entries[i].Money,
			ItemID:     entries[i].ItemID,
			Count:      entries[i].Count,
			OtherTab:   int32(entries[i].OtherTab),
		}
	}
	return result
}
