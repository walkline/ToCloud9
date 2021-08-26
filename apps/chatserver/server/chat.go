package server

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/chatserver/repo"
	"github.com/walkline/ToCloud9/apps/chatserver/sender"
	"github.com/walkline/ToCloud9/gen/chat/pb"
)

type ChatService struct {
	charRepo repo.CharactersRepo
}

func NewChatService(charRepo repo.CharactersRepo) *ChatService {
	return &ChatService{charRepo: charRepo}
}

func (s *ChatService) SendWhisperMessage(ctx context.Context, request *pb.SendWhisperMessageRequest) (*pb.SendWhisperMessageResponse, error) {
	char, err := s.charRepo.CharacterByRealmAndName(ctx, request.RealmID, request.ReceiverName)
	if err != nil {
		return nil, err
	}

	if char == nil {
		return &pb.SendWhisperMessageResponse{
			Api:    "v0.0.1",
			Status: pb.SendWhisperMessageResponse_CharacterNotFound,
		}, nil
	}

	log.Debug().Msgf("New whisper from '%s' to '%s'", request.SenderName, char.Name)

	err = char.MsgSender.SendWhisper(
		&sender.Character{
			RealmID: request.RealmID,
			GUID:    request.SenderGUID,
			Name:    request.SenderName,
			Race:    uint8(request.SenderRace),
		},
		&sender.Character{
			RealmID: request.RealmID,
			GUID:    char.GUID,
			Name:    char.Name,
			Race:    char.Race,
		},
		request.Language,
		request.Msg,
	)
	if err != nil {
		return nil, err
	}

	return &pb.SendWhisperMessageResponse{
		Api:          "v0.0.1",
		Status:       pb.SendWhisperMessageResponse_Ok,
		ReceiverGUID: char.GUID,
	}, nil
}
