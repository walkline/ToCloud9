package server

import (
	"context"
	"errors"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/chatserver"
	"github.com/walkline/ToCloud9/apps/chatserver/repo"
	"github.com/walkline/ToCloud9/apps/chatserver/sender"
	"github.com/walkline/ToCloud9/apps/chatserver/service"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	"github.com/walkline/ToCloud9/gen/chat/pb"
	wowguid "github.com/walkline/ToCloud9/shared/wow/guid"
)

var errWhisperReceiverAmbiguous = errors.New("whisper receiver ambiguous")

type ChatService struct {
	pb.UnimplementedChatServiceServer
	charRepo    repo.CharactersRepo
	channelMgr  *service.ChannelManager
	msgProducer sender.MsgProducer
	charClient  pbChar.CharactersServiceClient
	serviceID   string
}

func NewChatService(charRepo repo.CharactersRepo, channelMgr *service.ChannelManager, msgProducer sender.MsgProducer, serviceID string, charClient pbChar.CharactersServiceClient) *ChatService {
	return &ChatService{
		charRepo:    charRepo,
		channelMgr:  channelMgr,
		msgProducer: msgProducer,
		charClient:  charClient,
		serviceID:   serviceID,
	}
}

func (s *ChatService) SendWhisperMessage(ctx context.Context, request *pb.SendWhisperMessageRequest) (*pb.SendWhisperMessageResponse, error) {
	char, err := s.resolveWhisperReceiver(ctx, request.RealmID, request.ReceiverRealmID, request.ReceiverName)
	if errors.Is(err, errWhisperReceiverAmbiguous) {
		return &pb.SendWhisperMessageResponse{
			Api:    chatserver.Ver,
			Status: pb.SendWhisperMessageResponse_CharacterAmbiguous,
		}, nil
	}
	if err != nil {
		return nil, err
	}

	if char == nil {
		return &pb.SendWhisperMessageResponse{
			Api:    chatserver.Ver,
			Status: pb.SendWhisperMessageResponse_CharacterNotFound,
		}, nil
	}

	allowed, err := s.crossrealmWhisperAllowed(ctx, request, char)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return &pb.SendWhisperMessageResponse{
			Api:    chatserver.Ver,
			Status: pb.SendWhisperMessageResponse_CharacterNotFound,
		}, nil
	}

	log.Debug().
		Uint32("senderRealmID", request.RealmID).
		Uint32("receiverRealmID", char.RealmID).
		Msgf("New whisper from '%s' to '%s'", request.SenderName, char.Name)

	err = char.MsgSender.SendWhisper(
		&sender.Character{
			RealmID: request.RealmID,
			GUID:    request.SenderGUID,
			Name:    request.SenderName,
			Race:    uint8(request.SenderRace),
			Class:   uint8(request.SenderClass),
			Gender:  uint8(request.SenderGender),
			ChatTag: uint8(request.SenderChatTag),
		},
		&sender.Character{
			RealmID: char.RealmID,
			GUID:    char.GUID,
			Name:    char.Name,
			Race:    char.Race,
			Class:   char.Class,
			Gender:  char.Gender,
		},
		request.Language,
		request.Msg,
	)
	if err != nil {
		return nil, err
	}

	return &pb.SendWhisperMessageResponse{
		Api:             chatserver.Ver,
		Status:          pb.SendWhisperMessageResponse_Ok,
		ReceiverGUID:    wowguid.PlayerGUIDForRealm(request.RealmID, char.RealmID, char.GUID),
		ReceiverRealmID: char.RealmID,
		ReceiverName:    char.Name,
		ReceiverRace:    uint32(char.Race),
		ReceiverClass:   uint32(char.Class),
		ReceiverGender:  uint32(char.Gender),
	}, nil
}

func (s *ChatService) resolveWhisperReceiver(ctx context.Context, senderRealmID uint32, receiverRealmID uint32, receiverName string) (*repo.Character, error) {
	if receiverRealmID != 0 {
		return s.charRepo.CharacterByRealmAndName(ctx, receiverRealmID, receiverName)
	}

	char, err := s.charRepo.CharacterByRealmAndName(ctx, senderRealmID, receiverName)
	if err != nil {
		return nil, err
	}
	if char != nil {
		return char, nil
	}

	matches, err := s.charRepo.CharactersByName(ctx, receiverName)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, nil
	}
	if len(matches) > 1 {
		return nil, errWhisperReceiverAmbiguous
	}

	return matches[0], nil
}

func (s *ChatService) crossrealmWhisperAllowed(ctx context.Context, request *pb.SendWhisperMessageRequest, char *repo.Character) (bool, error) {
	if char.RealmID == request.RealmID {
		return true, nil
	}
	if request.GetGatewayValidatedGameplayCrossrealmWhisper() {
		log.Debug().
			Uint32("senderRealmID", request.RealmID).
			Uint32("receiverRealmID", char.RealmID).
			Uint32("senderAccountID", request.SenderAccountID).
			Uint32("receiverAccountID", char.AccountID).
			Str("receiverName", char.Name).
			Msg("Allowed crossrealm whisper through gateway-validated gameplay context")
		return true, nil
	}
	if request.SenderAccountID == 0 || char.AccountID == 0 {
		log.Debug().
			Uint32("senderRealmID", request.RealmID).
			Uint32("receiverRealmID", char.RealmID).
			Uint32("senderAccountID", request.SenderAccountID).
			Uint32("receiverAccountID", char.AccountID).
			Str("receiverName", char.Name).
			Msg("Denied crossrealm whisper without account identity")
		return false, nil
	}
	if s.charClient == nil {
		log.Warn().
			Uint32("senderRealmID", request.RealmID).
			Uint32("receiverRealmID", char.RealmID).
			Uint32("senderAccountID", request.SenderAccountID).
			Uint32("receiverAccountID", char.AccountID).
			Msg("Denied crossrealm whisper because characters service client is unavailable")
		return false, nil
	}

	res, err := s.charClient.AreRealIDFriends(ctx, &pbChar.AreRealIDFriendsRequest{
		Api:             chatserver.Ver,
		AccountID:       request.SenderAccountID,
		FriendAccountID: char.AccountID,
	})
	if err != nil {
		return false, err
	}

	return res.GetAccepted(), nil
}
