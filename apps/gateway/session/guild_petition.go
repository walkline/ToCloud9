package session

import (
	"context"

	root "github.com/walkline/ToCloud9/apps/gateway"
	eBroadcaster "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	pbGuild "github.com/walkline/ToCloud9/gen/guilds/pb"
	"github.com/walkline/ToCloud9/shared/events"
	wowguid "github.com/walkline/ToCloud9/shared/wow/guid"
)

func (s *GameSession) HandleOfferGuildPetition(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	reader.Uint32()
	petitionGUID := reader.Uint64()
	targetClientGUID := reader.Uint64()
	if reader.Error() != nil || s.character == nil {
		s.forwardPacketToWorldserver(p)
		return nil
	}

	petition, err := s.guildPetitionForGateway(ctx, petitionGUID)
	if err != nil {
		return err
	}
	if petition == nil {
		s.forwardPacketToWorldserver(p)
		return nil
	}

	if playerRealmIDOrDefault(s.guildHomeRealmID(), targetClientGUID) != s.guildHomeRealmID() {
		return nil
	}
	if s.charServiceClient == nil {
		s.forwardPacketToWorldserver(p)
		return nil
	}

	targetLowGUID := wowguid.PlayerLowGUID(targetClientGUID)
	target, err := s.onlineGuildPetitionTarget(ctx, targetLowGUID)
	if err != nil {
		return err
	}
	if target == nil {
		return nil
	}

	if target.CharGuildID != 0 {
		s.sendGuildCommandResult(guildCommandInvite, target.CharName, guildCommandResultAlreadyInGuildS)
		return nil
	}

	if !s.guildCanInviteRace(uint8(target.GetCharRace())) {
		s.sendGuildCommandResult(guildCommandCreate, "", guildCommandResultNotAllied)
		return nil
	}

	resp, err := s.guildServiceClient.OfferGuildPetition(ctx, &pbGuild.OfferGuildPetitionParams{
		Api:          root.Ver,
		RealmID:      s.guildHomeRealmID(),
		OwnerGUID:    s.character.GUID,
		TargetGUID:   target.CharGUID,
		TargetName:   target.CharName,
		PetitionGUID: petitionGUID,
	})
	if err != nil {
		return err
	}

	switch resp.GetStatus() {
	case pbGuild.OfferGuildPetitionResponse_TargetAlreadyInGuild:
		s.sendGuildCommandResult(guildCommandInvite, target.CharName, guildCommandResultAlreadyInGuildS)
	case pbGuild.OfferGuildPetitionResponse_TargetAlreadyInvited:
		s.sendGuildCommandResult(guildCommandInvite, target.CharName, guildCommandResultAlreadyInvitedToGuild)
	}

	return nil
}

func (s *GameSession) HandleSignGuildPetition(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	petitionGUID := reader.Uint64()
	reader.Uint8()
	if reader.Error() != nil || s.character == nil {
		s.forwardPacketToWorldserver(p)
		return nil
	}

	petition, err := s.guildPetitionForGateway(ctx, petitionGUID)
	if err != nil {
		return err
	}
	if petition == nil {
		s.forwardPacketToWorldserver(p)
		return nil
	}

	if !s.guildPetitionOwnerSameFaction(ctx, petition.GetOwnerGUID()) {
		s.sendGuildCommandResult(guildCommandCreate, "", guildCommandResultNotAllied)
		return nil
	}

	resp, err := s.guildServiceClient.SignGuildPetition(ctx, &pbGuild.SignGuildPetitionParams{
		Api:             root.Ver,
		RealmID:         s.guildHomeRealmID(),
		SignerGUID:      s.character.GUID,
		SignerName:      s.character.Name,
		SignerAccountID: s.character.AccountID,
		SignerGuildID:   s.character.GuildID,
		PetitionGUID:    petitionGUID,
	})
	if err != nil {
		return err
	}

	switch resp.GetStatus() {
	case pbGuild.SignGuildPetitionResponse_AlreadyInGuild:
		s.sendGuildCommandResult(guildCommandInvite, s.character.Name, guildCommandResultAlreadyInGuildS)
	case pbGuild.SignGuildPetitionResponse_AlreadyInvited:
		s.sendGuildCommandResult(guildCommandInvite, s.character.Name, guildCommandResultAlreadyInvitedToGuild)
	case pbGuild.SignGuildPetitionResponse_CantSignOwn:
		return nil
	default:
		s.sendGuildPetitionSignResult(petitionGUID, s.character.GUID, guildPetitionSignStatusToNative(resp.GetStatus()))
	}

	return nil
}

func (s *GameSession) HandleGuildPetitionShowSignatures(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	petitionGUID := reader.Uint64()
	if reader.Error() != nil {
		s.forwardPacketToWorldserver(p)
		return nil
	}

	petition, err := s.guildPetitionForGateway(ctx, petitionGUID)
	if err != nil {
		return err
	}
	if petition == nil {
		s.forwardPacketToWorldserver(p)
		return nil
	}

	s.sendGuildPetitionSignatures(petition)
	return nil
}

func (s *GameSession) HandleGuildPetitionQuery(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	reader.Uint32()
	petitionGUID := reader.Uint64()
	if reader.Error() != nil {
		s.forwardPacketToWorldserver(p)
		return nil
	}

	petition, err := s.guildPetitionForGateway(ctx, petitionGUID)
	if err != nil {
		return err
	}
	if petition == nil {
		s.forwardPacketToWorldserver(p)
		return nil
	}

	s.gameSocket.SendPacket(buildGuildPetitionQueryResponse(petition))
	return nil
}

func (s *GameSession) HandleEventGuildPetitionOffered(_ context.Context, e *eBroadcaster.Event) error {
	payload := e.Payload.(*events.GuildEventPetitionOfferedPayload)
	signatures := make([]*pbGuild.GuildPetitionSignature, len(payload.Signatures))
	for i := range payload.Signatures {
		signatures[i] = &pbGuild.GuildPetitionSignature{
			PlayerGUID:    payload.Signatures[i].PlayerGUID,
			PlayerAccount: payload.Signatures[i].PlayerAccount,
		}
	}

	s.sendGuildPetitionSignatures(&pbGuild.GuildPetition{
		PetitionGUID: payload.PetitionGUID,
		PetitionID:   payload.PetitionID,
		OwnerGUID:    payload.OwnerGUID,
		Name:         payload.GuildName,
		Type:         guildPetitionType,
		Signatures:   signatures,
	})
	return nil
}

func (s *GameSession) HandleEventGuildPetitionSigned(_ context.Context, e *eBroadcaster.Event) error {
	payload := e.Payload.(*events.GuildEventPetitionSignedPayload)
	s.sendGuildPetitionSignResult(payload.PetitionGUID, payload.SignerGUID, payload.NativeStatus)
	return nil
}

func (s *GameSession) guildPetitionForGateway(ctx context.Context, petitionGUID uint64) (*pbGuild.GuildPetition, error) {
	if s.guildServiceClient == nil {
		return nil, nil
	}

	resp, err := s.guildServiceClient.GetGuildPetition(ctx, &pbGuild.GetGuildPetitionParams{
		Api:          root.Ver,
		RealmID:      s.guildHomeRealmID(),
		PetitionGUID: petitionGUID,
	})
	if err != nil {
		return nil, err
	}

	if resp.GetPetition() == nil || resp.GetPetition().GetType() != guildPetitionType {
		return nil, nil
	}

	return resp.GetPetition(), nil
}

func (s *GameSession) onlineGuildPetitionTarget(ctx context.Context, targetLowGUID uint64) (*pbChar.ShortCharactersDataByGUIDsResponse_ShortCharData, error) {
	if s.charServiceClient == nil {
		return nil, nil
	}

	resp, err := s.charServiceClient.ShortOnlineCharactersDataByGUIDs(ctx, &pbChar.ShortCharactersDataByGUIDsRequest{
		Api:     root.Ver,
		RealmID: s.guildHomeRealmID(),
		GUIDs:   []uint64{targetLowGUID},
	})
	if err != nil {
		return nil, err
	}

	for _, character := range resp.GetCharacters() {
		if character.GetRealmID() == s.guildHomeRealmID() && character.GetCharGUID() == targetLowGUID && character.GetIsOnline() {
			return character, nil
		}
	}

	return nil, nil
}

func (s *GameSession) guildPetitionOwnerSameFaction(ctx context.Context, ownerGUID uint64) bool {
	if root.AllowTwoSideInteractionGuild {
		return true
	}
	if s == nil || s.character == nil || s.charServiceClient == nil {
		return false
	}

	resp, err := s.charServiceClient.CharacterByGUID(ctx, &pbChar.CharacterByGUIDRequest{
		Api:           root.Ver,
		RealmID:       s.guildHomeRealmID(),
		CharacterGUID: ownerGUID,
	})
	if err != nil || resp.GetCharacter() == nil || resp.GetCharacter().GetRealmID() != s.guildHomeRealmID() {
		return false
	}

	return guildSameFactionByRace(s.character.Race, uint8(resp.GetCharacter().GetCharRace()))
}

func (s *GameSession) sendGuildPetitionSignatures(petition *pbGuild.GuildPetition) {
	signatures := petition.GetSignatures()
	resp := packet.NewWriterWithSize(packet.SMsgPetitionShowSignatures, uint32(8+8+4+1+len(signatures)*12))
	resp.Uint64(petition.GetPetitionGUID())
	resp.Uint64(petition.GetOwnerGUID())
	resp.Uint32(petition.GetPetitionID())
	resp.Uint8(uint8(len(signatures)))
	for _, signature := range signatures {
		resp.Uint64(signature.GetPlayerGUID())
		resp.Uint32(0)
	}

	s.gameSocket.SendPacket(resp.ToPacket())
}

func (s *GameSession) sendGuildPetitionSignResult(petitionGUID uint64, signerGUID uint64, status uint32) {
	resp := packet.NewWriterWithSize(packet.SMsgPetitionSignResults, 8+8+4)
	resp.Uint64(petitionGUID)
	resp.Uint64(signerGUID)
	resp.Uint32(status)
	s.gameSocket.SendPacket(resp.ToPacket())
}

func (s *GameSession) sendGuildCommandResult(command int32, name string, result int32) {
	resp := packet.NewWriterWithSize(packet.SMsgGuildCommandResult, uint32(4+len(name)+1+4))
	resp.Int32(command)
	resp.String(name)
	resp.Int32(result)
	s.gameSocket.SendPacket(resp.ToPacket())
}

func (s *GameSession) forwardPacketToWorldserver(p *packet.Packet) {
	if s.worldSocket != nil {
		s.worldSocket.SendPacket(p)
	}
}

func guildPetitionSignStatusToNative(status pbGuild.SignGuildPetitionResponse_Status) uint32 {
	switch status {
	case pbGuild.SignGuildPetitionResponse_Ok:
		return petitionSignOK
	case pbGuild.SignGuildPetitionResponse_AlreadySigned:
		return petitionSignAlreadySigned
	case pbGuild.SignGuildPetitionResponse_AlreadyInGuild:
		return petitionSignAlreadyInGuild
	case pbGuild.SignGuildPetitionResponse_CantSignOwn:
		return petitionSignCantSignOwn
	default:
		return petitionSignNotServer
	}
}

func buildGuildPetitionQueryResponse(petition *pbGuild.GuildPetition) *packet.Packet {
	needed := petition.GetType()
	resp := packet.NewWriterWithSize(packet.SMsgPetitionQueryResponse, 0)
	resp.Uint32(petition.GetPetitionID())
	resp.Uint64(petition.GetOwnerGUID())
	resp.String(petition.GetName())
	resp.Uint8(0)
	resp.Uint32(needed)
	resp.Uint32(needed)
	resp.Uint32(0)
	resp.Uint32(0)
	resp.Uint32(0)
	resp.Uint32(0)
	resp.Uint32(0)
	resp.Uint16(0)
	resp.Uint32(0)
	resp.Uint32(0)
	resp.Uint32(0)
	for i := 0; i < 10; i++ {
		resp.Uint8(0)
	}
	resp.Uint32(0)
	resp.Uint32(0)
	return resp.ToPacket()
}
