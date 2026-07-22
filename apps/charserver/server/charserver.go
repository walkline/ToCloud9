package server

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/walkline/ToCloud9/apps/charserver/repo"
	"github.com/walkline/ToCloud9/apps/charserver/service"
	"github.com/walkline/ToCloud9/gen/characters/pb"
)

const (
	ver = "0.0.1"
)

type CharServer struct {
	pb.UnimplementedCharactersServiceServer
	repo           repo.Characters
	whoHandler     repo.WhoHandler
	itemsTemplate  repo.ItemsTemplate
	onlineChars    repo.CharactersOnline
	friendsService service.FriendsService
}

func NewCharServer(repo repo.Characters, onlineChars repo.CharactersOnline, whoHandler repo.WhoHandler, itemsTemplate repo.ItemsTemplate, friendsService service.FriendsService) pb.CharactersServiceServer {
	return &CharServer{
		repo:           repo,
		whoHandler:     whoHandler,
		itemsTemplate:  itemsTemplate,
		onlineChars:    onlineChars,
		friendsService: friendsService,
	}
}

func (c *CharServer) AcquireAccountSession(ctx context.Context, request *pb.AcquireAccountSessionRequest) (*pb.AcquireAccountSessionResponse, error) {
	if err := validateAccountSessionRequest(request.AccountID, request.OwnerToken, request.LeaseSeconds); err != nil {
		return nil, err
	}
	acquired, err := c.repo.AcquireAccountSession(ctx, request.RealmID, request.AccountID, request.OwnerToken, request.LeaseSeconds)
	return &pb.AcquireAccountSessionResponse{Acquired: acquired}, err
}

func (c *CharServer) RenewAccountSession(ctx context.Context, request *pb.RenewAccountSessionRequest) (*pb.RenewAccountSessionResponse, error) {
	if err := validateAccountSessionRequest(request.AccountID, request.OwnerToken, request.LeaseSeconds); err != nil {
		return nil, err
	}
	renewed, err := c.repo.RenewAccountSession(ctx, request.RealmID, request.AccountID, request.OwnerToken, request.LeaseSeconds)
	return &pb.RenewAccountSessionResponse{Renewed: renewed}, err
}

func (c *CharServer) ReleaseAccountSession(ctx context.Context, request *pb.ReleaseAccountSessionRequest) (*pb.ReleaseAccountSessionResponse, error) {
	if request.AccountID == 0 || len(request.OwnerToken) != 32 {
		return nil, status.Error(codes.InvalidArgument, "invalid account session owner")
	}
	released, err := c.repo.ReleaseAccountSession(ctx, request.RealmID, request.AccountID, request.OwnerToken)
	return &pb.ReleaseAccountSessionResponse{Released: released}, err
}

func validateAccountSessionRequest(accountID uint32, ownerToken string, leaseSeconds uint32) error {
	if accountID == 0 || len(ownerToken) != 32 || leaseSeconds < 5 || leaseSeconds > 300 {
		return status.Error(codes.InvalidArgument, "invalid account session lease")
	}
	return nil
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

func (c *CharServer) CharactersToLoginByGUID(ctx context.Context, request *pb.CharactersToLoginByGUIDRequest) (*pb.CharactersToLoginByGUIDResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint64("characterID", request.CharacterGUID).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled characters to login by GUID")
	}(time.Now())

	char, err := c.repo.CharacterToLogInByGUID(ctx, request.RealmID, request.CharacterGUID)
	if err != nil {
		return nil, err
	}
	if char != nil && request.AccountID != 0 && char.AccountID != request.AccountID {
		char = nil
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
		}
	}

	return &pb.CharactersToLoginByGUIDResponse{
		Api:       ver,
		Character: charResult,
	}, nil
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
		Api:     ver,
		Result:  result.Result,
		Status:  uint32(result.Status),
		Area:    result.Area,
		Level:   result.Level,
		ClassID: result.ClassID,
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
