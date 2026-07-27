package session

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	root "github.com/walkline/ToCloud9/apps/gateway"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	pbGuild "github.com/walkline/ToCloud9/gen/guilds/pb"
	pbWorld "github.com/walkline/ToCloud9/gen/worldserver/pb"
)

// Petition turn-in results carried by SMSG_TURN_IN_PETITION_RESULTS. The client
// localizes the displayed message from these codes, so no server-side
// translation is needed.
const (
	petitionTurnOK                 = 0 // PETITION_TURN_OK
	petitionTurnAlreadyInGuild     = 2 // PETITION_TURN_ALREADY_IN_GUILD
	petitionTurnNeedMoreSignatures = 4 // PETITION_TURN_NEED_MORE_SIGNATURES
)

// sendTurnInPetitionResult sends SMSG_TURN_IN_PETITION_RESULTS so the client
// closes the petition dialog and renders the localized result.
func (s *GameSession) sendTurnInPetitionResult(result uint32) {
	w := packet.NewWriterWithSize(packet.SMsgTurnInPetitionResults, 4)
	w.Uint32(result)
	s.gameSocket.Send(w)
}

// HandleTurnInPetition handles CMSG_TURN_IN_PETITION for guild charters. The
// worldserver validates the petition (charter item, ownership, signatures) over
// gRPC; on success the guild service creates the guild with the signatories as
// members, and worldservers populate their state from the guild.created event.
// Arena charters keep the in-process worldserver flow and are forwarded as is.
func (s *GameSession) HandleTurnInPetition(ctx context.Context, p *packet.Packet) error {
	petitionGUID := p.Reader().Uint64()

	gameClient, err := s.gameServerGRPCConnMgr.GRPCConnByGameServerAddress(s.worldSocket.Address())
	if err != nil {
		return fmt.Errorf("can't get gameServiceClient, err: %w", err)
	}

	checkResp, err := gameClient.CanTurnInGuildPetition(ctx, &pbWorld.CanTurnInGuildPetitionRequest{
		Api:              root.SupportedGameServerVer,
		PlayerGUID:       s.character.GUID,
		PetitionItemGUID: petitionGUID,
	})
	if err != nil {
		if status.Code(err) == codes.Unimplemented {
			// The worldserver predates the petition validation endpoint
			// (mixed versions in the cluster): fall back to the in-process
			// turn-in flow instead of silently dropping the packet.
			s.worldSocket.SendPacket(p)
			return nil
		}
		return fmt.Errorf("can't validate guild petition, err: %w", err)
	}

	switch checkResp.Status {
	case pbWorld.CanTurnInGuildPetitionResponse_Ok:
	case pbWorld.CanTurnInGuildPetitionResponse_NotGuildPetition:
		// Arena charters are handled in-process by the worldserver.
		s.worldSocket.SendPacket(p)
		return nil
	case pbWorld.CanTurnInGuildPetitionResponse_AlreadyInGuild:
		s.sendTurnInPetitionResult(petitionTurnAlreadyInGuild)
		return nil
	case pbWorld.CanTurnInGuildPetitionResponse_NeedMoreSignatures:
		s.sendTurnInPetitionResult(petitionTurnNeedMoreSignatures)
		return nil
	default:
		// PlayerNotFound, PetitionNotFound, NotPetitionOwner: the core handler
		// silently ignores these cases, do the same.
		return nil
	}

	createResp, err := s.guildServiceClient.CreateGuild(ctx, &pbGuild.CreateGuildParams{
		Api:            root.Ver,
		RealmID:        root.RealmID,
		LeaderGUID:     s.character.GUID,
		Name:           checkResp.GuildName,
		SignatoryGUIDs: checkResp.SignatoryGUIDs,
	})
	if err != nil {
		switch status.Code(err) {
		case codes.AlreadyExists:
			s.sendGuildCommandResult(guildCommandCreate, checkResp.GuildName, guildErrNameExistsS)
			return nil
		case codes.FailedPrecondition:
			s.sendTurnInPetitionResult(petitionTurnAlreadyInGuild)
			return nil
		case codes.InvalidArgument:
			s.sendGuildCommandResult(guildCommandCreate, checkResp.GuildName, guildErrNameInvalid)
			return nil
		}
		return fmt.Errorf("can't create guild, err: %w", err)
	}

	s.character.GuildID = uint32(createResp.GuildID)

	s.sendGuildCommandResult(guildCommandCreate, checkResp.GuildName, guildErrCommandSuccess)
	s.sendTurnInPetitionResult(petitionTurnOK)

	return nil
}
