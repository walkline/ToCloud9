package server

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/charserver/repo"
	"github.com/walkline/ToCloud9/apps/charserver/service"
	"github.com/walkline/ToCloud9/gen/characters/pb"
	"github.com/walkline/ToCloud9/shared/events"
)

const (
	ver = "0.0.1"
)

type CharServer struct {
	pb.UnimplementedCharactersServiceServer
	repo           repo.Characters
	arenaTeams     repo.ArenaTeams
	whoHandler     repo.WhoHandler
	itemsTemplate  repo.ItemsTemplate
	onlineChars    repo.CharactersOnline
	friendsService service.FriendsService
	eventsProducer events.CharactersServiceProducer
	arenaInvitesMu sync.Mutex
	arenaInvites   map[arenaTeamInviteKey]arenaTeamInvite
}

func NewCharServer(repo repo.Characters, arenaTeams repo.ArenaTeams, onlineChars repo.CharactersOnline, whoHandler repo.WhoHandler, itemsTemplate repo.ItemsTemplate, friendsService service.FriendsService, eventsProducer events.CharactersServiceProducer) pb.CharactersServiceServer {
	return &CharServer{
		repo:           repo,
		arenaTeams:     arenaTeams,
		whoHandler:     whoHandler,
		itemsTemplate:  itemsTemplate,
		onlineChars:    onlineChars,
		friendsService: friendsService,
		eventsProducer: eventsProducer,
		arenaInvites:   make(map[arenaTeamInviteKey]arenaTeamInvite),
	}
}

type arenaTeamInviteKey struct {
	realmID    uint32
	playerGUID uint64
}

type arenaTeamInvite struct {
	arenaTeamID uint32
	inviterGUID uint64
	inviterName string
	teamName    string
}

func (c *CharServer) CharactersToLoginForAccount(ctx context.Context, request *pb.CharactersToLoginForAccountRequest) (*pb.CharactersToLoginForAccountResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint32("accountID", request.AccountID).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled characters to login request")
	}(time.Now())

	chars, err := c.repo.ListCharactersToLogIn(ctx, request.RealmID, request.AccountID)
	if err != nil {
		return nil, err
	}

	result := make([]*pb.LogInCharacter, 0, len(chars))
	for _, char := range chars {
		equipments := []*pb.EquipmentDisplay{}
		for _, itemID := range char.Equipments {
			if itemID == 0 {
				equipments = append(equipments, &pb.EquipmentDisplay{})
				continue
			}

			item, err := c.itemsTemplate.TemplateByID(itemID)
			if err != nil {
				return nil, err
			}

			equipments = append(equipments, &pb.EquipmentDisplay{
				DisplayInfoID: item.DisplayID,
				InventoryType: uint32(item.InventoryType),
				EnchantmentID: 0,
			})
		}

		item := &pb.LogInCharacter{
			GUID:        char.GUID,
			Name:        char.Name,
			Race:        uint32(char.Race),
			Class:       uint32(char.Class),
			Gender:      uint32(char.Gender),
			Skin:        uint32(char.Skin),
			Face:        uint32(char.Face),
			HairStyle:   uint32(char.HairStyle),
			HairColor:   uint32(char.HairColor),
			FacialStyle: uint32(char.FacialStyle),
			Level:       uint32(char.Level),
			Zone:        char.Zone,
			Map:         char.Map,
			PositionX:   char.PositionX,
			PositionY:   char.PositionY,
			PositionZ:   char.PositionZ,
			GuildID:     char.GuildID,
			PlayerFlags: char.PlayerFlags,
			AtLogin:     uint32(char.AtLoginFlags),
			PetEntry:    char.PetEntry,
			PetModelID:  char.PetModelID,
			PetLevel:    uint32(char.PetLevel),
			Equipments:  equipments,
			Banned:      char.Banned,
			AccountID:   char.AccountID,
			ExtraFlags:  char.ExtraFlags,
		}
		result = append(result, item)
	}

	return &pb.CharactersToLoginForAccountResponse{
		Api:        ver,
		Characters: result,
	}, nil
}

func (c *CharServer) AccountDataForAccount(ctx context.Context, request *pb.AccountDataForAccountRequest) (*pb.AccountDataForAccountResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint32("accountID", request.AccountID).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled account data request")
	}(time.Now())

	accountData, err := c.repo.AccountDataForAccountID(ctx, request.RealmID, request.AccountID)
	if err != nil {
		return nil, err
	}

	res := make([]*pb.AccountData, 0, len(accountData))
	for _, item := range accountData {
		res = append(res, &pb.AccountData{
			Type: uint32(item.Type),
			Time: item.Time,
			Data: item.Data,
		})
	}

	return &pb.AccountDataForAccountResponse{
		Api:         ver,
		AccountData: res,
	}, nil
}

func (c *CharServer) UpdateAccountDataForAccount(ctx context.Context, request *pb.UpdateAccountDataForAccountRequest) (*pb.UpdateAccountDataForAccountResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint32("accountID", request.AccountID).
			Uint32("realmID", request.RealmID).
			Uint32("type", request.Type).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled account data update")
	}(time.Now())

	err := c.repo.UpdateAccountDataForAccountID(ctx, request.RealmID, request.AccountID, repo.AccountData{
		Type: uint8(request.Type),
		Time: request.Time,
		Data: request.Data,
	})
	if err != nil {
		return nil, err
	}

	return &pb.UpdateAccountDataForAccountResponse{
		Api: ver,
	}, nil
}

func (c *CharServer) CharactersToLoginByGUID(ctx context.Context, request *pb.CharactersToLoginByGUIDRequest) (*pb.CharactersToLoginByGUIDResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint64("characterID", request.CharacterGUID).
			Uint32("accountID", request.AccountID).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled characters to login by GUID")
	}(time.Now())

	char, err := c.repo.CharacterToLogInByGUID(ctx, request.RealmID, request.AccountID, request.CharacterGUID)
	if err != nil {
		return nil, err
	}
	var charResult *pb.LogInCharacter
	if char != nil {
		equipments := []*pb.EquipmentDisplay{}
		for _, itemID := range char.Equipments {
			if itemID == 0 {
				equipments = append(equipments, &pb.EquipmentDisplay{})
				continue
			}

			item, err := c.itemsTemplate.TemplateByID(itemID)
			if err != nil {
				return nil, err
			}

			equipments = append(equipments, &pb.EquipmentDisplay{
				DisplayInfoID: item.DisplayID,
				InventoryType: uint32(item.InventoryType),
				EnchantmentID: 0,
			})
		}

		charResult = &pb.LogInCharacter{
			GUID:        char.GUID,
			Name:        char.Name,
			Race:        uint32(char.Race),
			Class:       uint32(char.Class),
			Gender:      uint32(char.Gender),
			Skin:        uint32(char.Skin),
			Face:        uint32(char.Face),
			HairStyle:   uint32(char.HairStyle),
			HairColor:   uint32(char.HairColor),
			FacialStyle: uint32(char.FacialStyle),
			Level:       uint32(char.Level),
			Zone:        char.Zone,
			Map:         char.Map,
			PositionX:   char.PositionX,
			PositionY:   char.PositionY,
			PositionZ:   char.PositionZ,
			GuildID:     char.GuildID,
			PlayerFlags: char.PlayerFlags,
			AtLogin:     uint32(char.AtLoginFlags),
			PetEntry:    char.PetEntry,
			PetModelID:  char.PetModelID,
			PetLevel:    uint32(char.PetLevel),
			Equipments:  equipments,
			Banned:      char.Banned,
			AccountID:   char.AccountID,
			ExtraFlags:  char.ExtraFlags,
		}
	}

	return &pb.CharactersToLoginByGUIDResponse{
		Api:       ver,
		Character: charResult,
	}, nil
}

func (c *CharServer) RecordLfgDungeonRoute(ctx context.Context, request *pb.RecordLfgDungeonRouteRequest) (*pb.RecordLfgDungeonRouteResponse, error) {
	route := request.GetRoute()
	if route == nil {
		return &pb.RecordLfgDungeonRouteResponse{Api: ver}, nil
	}

	err := c.repo.RecordLfgDungeonRoute(ctx, repo.LfgDungeonRoute{
		RealmID:               route.GetRealmID(),
		PlayerGUID:            route.GetPlayerGUID(),
		DungeonEntry:          route.GetDungeonEntry(),
		MapID:                 route.GetMapID(),
		Difficulty:            uint8(route.GetDifficulty()),
		OwnerRealmID:          route.GetOwnerRealmID(),
		IsCrossRealm:          route.GetIsCrossRealm(),
		RequiresBoundInstance: route.GetRequiresBoundInstance(),
		InstanceID:            route.GetInstanceID(),
	})
	if err != nil {
		return nil, err
	}
	return &pb.RecordLfgDungeonRouteResponse{Api: ver}, nil
}

func (c *CharServer) ConfirmLfgDungeonRouteEntered(ctx context.Context, request *pb.ConfirmLfgDungeonRouteEnteredRequest) (*pb.ConfirmLfgDungeonRouteEnteredResponse, error) {
	route, err := c.repo.ConfirmLfgDungeonRouteEntered(ctx, request.GetRealmID(), request.GetPlayerGUID(), request.GetMapID(), uint8(request.GetDifficulty()), request.GetInstanceID())
	if err != nil {
		return nil, err
	}
	return &pb.ConfirmLfgDungeonRouteEnteredResponse{
		Api:   ver,
		Route: lfgDungeonRouteToProto(route),
	}, nil
}

func (c *CharServer) ClearUnboundLfgDungeonRoute(ctx context.Context, request *pb.ClearUnboundLfgDungeonRouteRequest) (*pb.ClearUnboundLfgDungeonRouteResponse, error) {
	if err := c.repo.ClearUnboundLfgDungeonRoute(ctx, request.GetRealmID(), request.GetPlayerGUID(), request.GetMapID()); err != nil {
		return nil, err
	}
	return &pb.ClearUnboundLfgDungeonRouteResponse{Api: ver}, nil
}

func (c *CharServer) GetLfgDungeonRoute(ctx context.Context, request *pb.GetLfgDungeonRouteRequest) (*pb.GetLfgDungeonRouteResponse, error) {
	route, err := c.repo.LfgDungeonRouteForPlayer(ctx, request.GetRealmID(), request.GetPlayerGUID(), request.GetMapID())
	if err != nil {
		return nil, err
	}
	if route == nil {
		return &pb.GetLfgDungeonRouteResponse{Api: ver}, nil
	}

	available := !route.RequiresBoundInstance || route.BoundInstanceID != 0
	return &pb.GetLfgDungeonRouteResponse{
		Api:       ver,
		Found:     true,
		Available: available,
		Route:     lfgDungeonRouteToProto(route),
	}, nil
}

func lfgDungeonRouteToProto(route *repo.LfgDungeonRoute) *pb.LfgDungeonRoute {
	if route == nil {
		return nil
	}
	return &pb.LfgDungeonRoute{
		RealmID:               route.RealmID,
		PlayerGUID:            route.PlayerGUID,
		DungeonEntry:          route.DungeonEntry,
		MapID:                 route.MapID,
		Difficulty:            uint32(route.Difficulty),
		OwnerRealmID:          route.OwnerRealmID,
		IsCrossRealm:          route.IsCrossRealm,
		RequiresBoundInstance: route.RequiresBoundInstance,
		InstanceID:            route.InstanceID,
		BoundInstanceID:       route.BoundInstanceID,
	}
}

func (c *CharServer) WhoQuery(ctx context.Context, request *pb.WhoQueryRequest) (*pb.WhoQueryResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint64("characterID", request.CharacterGUID).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled who query")
	}(time.Now())

	chars, err := c.whoHandler.WhoRequest(ctx, request.RealmID, request.CharacterGUID, repo.CharactersWhoQuery{
		LvlMin:    uint8(request.LvlMin),
		LvlMax:    uint8(request.LvlMax),
		ClassMask: request.ClassMask,
		RaceMask:  request.RaceMask,
		Zones:     request.Zones,
		Strings:   request.Strings,
	})
	if err != nil {
		return nil, err
	}
	items := make([]*pb.WhoQueryResponse_WhoItem, 0, 50)
	for i := 0; i < len(chars) && i < 50; i++ {
		items = append(items, &pb.WhoQueryResponse_WhoItem{
			Guid:   chars[i].CharGUID,
			Name:   chars[i].CharName,
			Guild:  "", // TODO: add guilds support.
			Lvl:    uint32(chars[i].CharLevel),
			Class:  uint32(chars[i].CharClass),
			Race:   uint32(chars[i].CharRace),
			Gender: uint32(chars[i].CharGender),
			ZoneID: chars[i].CharZone,
		})
	}

	return &pb.WhoQueryResponse{
		Api:            ver,
		TotalFound:     uint32(len(chars)),
		ItemsToDisplay: items,
	}, nil
}

func (c *CharServer) CharacterOnlineByName(ctx context.Context, request *pb.CharacterOnlineByNameRequest) (*pb.CharacterOnlineByNameResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Str("name", request.CharacterName).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled character online by name")
	}(time.Now())

	char, err := c.onlineChars.OneByRealmAndName(ctx, request.RealmID, request.CharacterName)
	if err != nil {
		return nil, err
	}

	if char == nil {
		return &pb.CharacterOnlineByNameResponse{
			Api:       ver,
			Character: nil,
		}, nil
	}

	return &pb.CharacterOnlineByNameResponse{
		Api: ver,
		Character: &pb.CharacterOnlineByNameResponse_Char{
			RealmID:     char.RealmID,
			GatewayID:   char.GatewayID,
			CharGUID:    char.CharGUID,
			CharName:    char.CharName,
			CharRace:    uint32(char.CharRace),
			CharClass:   uint32(char.CharClass),
			CharGender:  uint32(char.CharGender),
			CharLvl:     uint32(char.CharLevel),
			CharZone:    char.CharZone,
			CharMap:     char.CharMap,
			CharGuildID: uint64(char.CharGuildID),
			AccountID:   char.AccountID,
		},
	}, nil
}

func (c *CharServer) CharacterByName(ctx context.Context, request *pb.CharacterByNameRequest) (*pb.CharacterByNameResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Str("name", request.CharacterName).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled character by name")
	}(time.Now())

	char, err := c.onlineChars.OneByRealmAndName(ctx, request.RealmID, request.CharacterName)
	if err != nil {
		return nil, err
	}

	if char != nil {
		return &pb.CharacterByNameResponse{
			Api: ver,
			Character: &pb.CharacterByNameResponse_Char{
				RealmID:     char.RealmID,
				IsOnline:    true,
				GatewayID:   char.GatewayID,
				CharGUID:    char.CharGUID,
				CharName:    char.CharName,
				CharRace:    uint32(char.CharRace),
				CharClass:   uint32(char.CharClass),
				CharGender:  uint32(char.CharGender),
				CharLvl:     uint32(char.CharLevel),
				CharZone:    char.CharZone,
				CharMap:     char.CharMap,
				CharGuildID: uint64(char.CharGuildID),
				AccountID:   char.AccountID,
			},
		}, nil
	}

	char, err = c.repo.CharacterByName(ctx, request.RealmID, request.CharacterName)
	if err != nil {
		return nil, err
	}

	if char == nil {
		return &pb.CharacterByNameResponse{
			Api:       ver,
			Character: nil,
		}, nil
	}

	return &pb.CharacterByNameResponse{
		Api: ver,
		Character: &pb.CharacterByNameResponse_Char{
			RealmID:     char.RealmID,
			IsOnline:    false,
			GatewayID:   "",
			CharGUID:    char.CharGUID,
			CharName:    char.CharName,
			CharRace:    uint32(char.CharRace),
			CharClass:   uint32(char.CharClass),
			CharGender:  uint32(char.CharGender),
			CharLvl:     uint32(char.CharLevel),
			CharZone:    char.CharZone,
			CharMap:     char.CharMap,
			CharGuildID: uint64(char.CharGuildID),
			AccountID:   char.AccountID,
		},
	}, nil
}

func (c *CharServer) CharacterByGUID(ctx context.Context, request *pb.CharacterByGUIDRequest) (*pb.CharacterByGUIDResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint64("guid", request.CharacterGUID).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled character by guid")
	}(time.Now())

	chars, err := c.onlineChars.CharactersByRealmAndGUIDs(ctx, request.RealmID, []uint64{request.CharacterGUID})
	if err != nil {
		return nil, err
	}

	if len(chars) > 0 {
		char := chars[0]
		return &pb.CharacterByGUIDResponse{
			Api: ver,
			Character: &pb.CharacterByNameResponse_Char{
				RealmID:     char.RealmID,
				IsOnline:    true,
				GatewayID:   char.GatewayID,
				CharGUID:    char.CharGUID,
				CharName:    char.CharName,
				CharRace:    uint32(char.CharRace),
				CharClass:   uint32(char.CharClass),
				CharGender:  uint32(char.CharGender),
				CharLvl:     uint32(char.CharLevel),
				CharZone:    char.CharZone,
				CharMap:     char.CharMap,
				CharGuildID: uint64(char.CharGuildID),
				AccountID:   char.AccountID,
			},
		}, nil
	}

	char, err := c.repo.CharacterByGUID(ctx, request.RealmID, request.CharacterGUID)
	if err != nil {
		return nil, err
	}
	if char == nil {
		return &pb.CharacterByGUIDResponse{
			Api:       ver,
			Character: nil,
		}, nil
	}

	return &pb.CharacterByGUIDResponse{
		Api: ver,
		Character: &pb.CharacterByNameResponse_Char{
			RealmID:     char.RealmID,
			IsOnline:    false,
			GatewayID:   "",
			CharGUID:    char.CharGUID,
			CharName:    char.CharName,
			CharRace:    uint32(char.CharRace),
			CharClass:   uint32(char.CharClass),
			CharGender:  uint32(char.CharGender),
			CharLvl:     uint32(char.CharLevel),
			CharZone:    char.CharZone,
			CharMap:     char.CharMap,
			CharGuildID: uint64(char.CharGuildID),
			AccountID:   char.AccountID,
		},
	}, nil
}

func (c *CharServer) ShortOnlineCharactersDataByGUIDs(ctx context.Context, request *pb.ShortCharactersDataByGUIDsRequest) (*pb.ShortCharactersDataByGUIDsResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Int("guidsSize", len(request.GUIDs)).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled short characters by guids")
	}(time.Now())

	chars, err := c.onlineChars.CharactersByRealmAndGUIDs(ctx, request.RealmID, request.GUIDs)
	if err != nil {
		return nil, err
	}

	res := make([]*pb.ShortCharactersDataByGUIDsResponse_ShortCharData, len(chars))
	for i, char := range chars {
		res[i] = &pb.ShortCharactersDataByGUIDsResponse_ShortCharData{
			RealmID:     char.RealmID,
			IsOnline:    true,
			GatewayID:   char.GatewayID,
			CharGUID:    char.CharGUID,
			CharName:    char.CharName,
			CharRace:    uint32(char.CharRace),
			CharClass:   uint32(char.CharClass),
			CharGender:  uint32(char.CharGender),
			CharLvl:     uint32(char.CharLevel),
			CharZone:    char.CharZone,
			CharMap:     char.CharMap,
			CharGuildID: uint64(char.CharGuildID),
			AccountID:   char.AccountID,
		}
	}

	return &pb.ShortCharactersDataByGUIDsResponse{
		Api:        ver,
		Characters: res,
	}, nil
}

func (c *CharServer) SavePlayerPosition(ctx context.Context, request *pb.SavePlayerPositionRequest) (*pb.SavePlayerPositionResponse, error) {
	err := c.repo.SaveCharacterPosition(ctx, request.RealmID, request.CharGUID, request.MapID, request.X, request.Y, request.Z, request.O)
	if err != nil {
		return nil, err
	}

	return &pb.SavePlayerPositionResponse{
		Api: ver,
	}, nil
}

func (c *CharServer) GetFriendsList(ctx context.Context, request *pb.GetFriendsListRequest) (*pb.GetFriendsListResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint64("playerGUID", request.PlayerGUID).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled get friends list")
	}(time.Now())

	friendsList, err := c.friendsService.GetFriendsList(ctx, request.RealmID, request.PlayerGUID)
	if err != nil {
		return nil, err
	}

	friends := make([]*pb.GetFriendsListResponse_Friend, 0, len(friendsList.Friends))
	for _, friend := range friendsList.Friends {
		friends = append(friends, &pb.GetFriendsListResponse_Friend{
			RealmID: friend.RealmID,
			Guid:    friend.GUID,
			Note:    friend.Note,
			Status:  uint32(friend.Status),
			Area:    friend.Area,
			Level:   friend.Level,
			ClassID: friend.ClassID,
		})
	}

	ignored := make([]*pb.GetFriendsListResponse_IgnoredPlayer, 0, len(friendsList.Ignored))
	for _, guid := range friendsList.Ignored {
		ignored = append(ignored, &pb.GetFriendsListResponse_IgnoredPlayer{
			Guid: guid,
		})
	}

	return &pb.GetFriendsListResponse{
		Api:     ver,
		Friends: friends,
		Ignored: ignored,
	}, nil
}

func (c *CharServer) AddFriend(ctx context.Context, request *pb.AddFriendRequest) (*pb.AddFriendResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint64("playerGUID", request.PlayerGUID).
			Uint64("friendGUID", request.FriendGUID).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled add friend")
	}(time.Now())

	result, err := c.friendsService.AddFriend(ctx, request.RealmID, request.PlayerGUID, request.FriendGUID, request.FriendName, request.Note)
	if err != nil {
		return nil, err
	}

	return &pb.AddFriendResponse{
		Api:        ver,
		Result:     result.Result,
		Status:     uint32(result.Status),
		Area:       result.Area,
		Level:      result.Level,
		ClassID:    result.ClassID,
		FriendGUID: result.FriendGUID,
		Pending:    result.Pending,
		Accepted:   result.Accepted,
	}, nil
}

func (c *CharServer) AddRealIDFriendByEmail(ctx context.Context, request *pb.AddRealIDFriendByEmailRequest) (*pb.AddRealIDFriendByEmailResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint64("playerGUID", request.PlayerGUID).
			Uint32("accountID", request.AccountID).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled add real id friend by email")
	}(time.Now())

	result, err := c.friendsService.AddRealIDFriendByEmail(ctx, request.RealmID, request.PlayerGUID, request.AccountID, request.Email, request.Note)
	if err != nil {
		return nil, err
	}

	return &pb.AddRealIDFriendByEmailResponse{
		Api:        ver,
		Result:     result.Result,
		Status:     uint32(result.Status),
		Area:       result.Area,
		Level:      result.Level,
		ClassID:    result.ClassID,
		FriendGUID: result.FriendGUID,
		Pending:    result.Pending,
		Accepted:   result.Accepted,
	}, nil
}

func (c *CharServer) AcceptRealIDFriend(ctx context.Context, request *pb.AcceptRealIDFriendRequest) (*pb.AcceptRealIDFriendResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint64("playerGUID", request.PlayerGUID).
			Uint32("accountID", request.AccountID).
			Uint32("requesterAccountID", request.RequesterAccountID).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled accept real id friend")
	}(time.Now())

	result, err := c.friendsService.AcceptRealIDFriend(ctx, request.RealmID, request.PlayerGUID, request.AccountID, request.RequesterAccountID, request.Note)
	if err != nil {
		return nil, err
	}

	return &pb.AcceptRealIDFriendResponse{
		Api:        ver,
		Result:     result.Result,
		Status:     uint32(result.Status),
		Area:       result.Area,
		Level:      result.Level,
		ClassID:    result.ClassID,
		FriendGUID: result.FriendGUID,
		Pending:    result.Pending,
		Accepted:   result.Accepted,
	}, nil
}

func (c *CharServer) AreRealIDFriends(ctx context.Context, request *pb.AreRealIDFriendsRequest) (*pb.AreRealIDFriendsResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint32("accountID", request.AccountID).
			Uint32("friendAccountID", request.FriendAccountID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled are real id friends")
	}(time.Now())

	accepted, err := c.friendsService.AreRealIDFriends(ctx, request.AccountID, request.FriendAccountID)
	if err != nil {
		return nil, err
	}

	return &pb.AreRealIDFriendsResponse{
		Api:      ver,
		Accepted: accepted,
	}, nil
}

func (c *CharServer) RemoveFriend(ctx context.Context, request *pb.RemoveFriendRequest) (*pb.RemoveFriendResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint64("playerGUID", request.PlayerGUID).
			Uint64("friendGUID", request.FriendGUID).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled remove friend")
	}(time.Now())

	err := c.friendsService.RemoveFriend(ctx, request.RealmID, request.PlayerGUID, request.FriendGUID)
	if err != nil {
		return nil, err
	}

	return &pb.RemoveFriendResponse{
		Api: ver,
	}, nil
}

func (c *CharServer) SetFriendNote(ctx context.Context, request *pb.SetFriendNoteRequest) (*pb.SetFriendNoteResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint64("playerGUID", request.PlayerGUID).
			Uint64("friendGUID", request.FriendGUID).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled set friend note")
	}(time.Now())

	err := c.friendsService.SetFriendNote(ctx, request.RealmID, request.PlayerGUID, request.FriendGUID, request.Note)
	if err != nil {
		return nil, err
	}

	return &pb.SetFriendNoteResponse{
		Api: ver,
	}, nil
}

func (c *CharServer) AddIgnore(ctx context.Context, request *pb.AddIgnoreRequest) (*pb.AddIgnoreResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint64("playerGUID", request.PlayerGUID).
			Uint64("ignoredGUID", request.IgnoredGUID).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled add ignore")
	}(time.Now())

	result, err := c.friendsService.AddIgnore(ctx, request.RealmID, request.PlayerGUID, request.IgnoredGUID)
	if err != nil {
		return nil, err
	}

	return &pb.AddIgnoreResponse{
		Api:    ver,
		Result: result,
	}, nil
}

func (c *CharServer) RemoveIgnore(ctx context.Context, request *pb.RemoveIgnoreRequest) (*pb.RemoveIgnoreResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint64("playerGUID", request.PlayerGUID).
			Uint64("ignoredGUID", request.IgnoredGUID).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled remove ignore")
	}(time.Now())

	err := c.friendsService.RemoveIgnore(ctx, request.RealmID, request.PlayerGUID, request.IgnoredGUID)
	if err != nil {
		return nil, err
	}

	return &pb.RemoveIgnoreResponse{
		Api: ver,
	}, nil
}

func (c *CharServer) NotifyStatusChange(ctx context.Context, request *pb.NotifyStatusChangeRequest) (*pb.NotifyStatusChangeResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint64("playerGUID", request.PlayerGUID).
			Uint32("status", request.Status).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled notify status change")
	}(time.Now())

	err := c.friendsService.NotifyStatusChange(ctx, request.RealmID, request.PlayerGUID, uint8(request.Status), request.Area, request.Level, request.ClassID)
	if err != nil {
		return nil, err
	}

	return &pb.NotifyStatusChangeResponse{
		Api: ver,
	}, nil
}

func (c *CharServer) GetOnlineCharacters(ctx context.Context, request *pb.GetOnlineCharactersRequest) (*pb.GetOnlineCharactersResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled get online characters")
	}(time.Now())

	guids, err := c.onlineChars.AllGUIDsByRealm(ctx, request.RealmID)
	if err != nil {
		return nil, err
	}

	return &pb.GetOnlineCharactersResponse{
		Api:            ver,
		CharacterGUIDs: guids,
		TotalCount:     uint32(len(guids)),
	}, nil
}
