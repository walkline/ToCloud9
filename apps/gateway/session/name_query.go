package session

import (
	"context"

	root "github.com/walkline/ToCloud9/apps/gateway"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	pbGroup "github.com/walkline/ToCloud9/gen/group/pb"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

func (s *GameSession) HandleNameQuery(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	queryGUID := reader.Uint64()
	if err := reader.Error(); err != nil {
		return err
	}

	memberGUID := playerDBGUIDFromClientGUID(queryGUID)
	if memberGUID == 0 || s.character == nil || s.groupServiceClient == nil || s.charServiceClient == nil {
		if s.sendForeignNameQueryResponse(ctx, queryGUID) {
			return nil
		}
		s.forwardNameQueryToWorld(p)
		return nil
	}

	groupResp, err := s.groupServiceClient.GetGroupByMember(ctx, &pbGroup.GetGroupByMemberRequest{
		Api:     root.SupportedGroupServiceVer,
		RealmID: root.RealmID,
		Player:  s.character.GUID,
	})
	if err != nil || groupResp == nil || groupResp.Group == nil {
		if s.sendForeignNameQueryResponse(ctx, queryGUID) {
			return nil
		}
		s.forwardNameQueryToWorld(p)
		return nil
	}

	groupRealmID := groupHomeRealmIDFromPB(groupResp.Group)
	var member *pbGroup.GetGroupResponse_GroupMember
	for _, candidate := range groupResp.Group.Members {
		if samePlayerGUID(groupRealmID, candidate.Guid, root.RealmID, memberGUID) {
			member = candidate
			break
		}
	}
	if member == nil || member.Name == "" {
		if s.sendForeignNameQueryResponse(ctx, queryGUID) {
			return nil
		}
		s.forwardNameQueryToWorld(p)
		return nil
	}

	memberRealmID := groupMemberRealmID(groupRealmID, member)
	charResp, err := s.charServiceClient.CharacterByName(ctx, &pbChar.CharacterByNameRequest{
		Api:           root.Ver,
		RealmID:       memberRealmID,
		CharacterName: member.Name,
	})
	if err != nil || charResp == nil || charResp.Character == nil ||
		!samePlayerGUID(memberRealmID, charResp.Character.CharGUID, groupRealmID, member.Guid) {
		if s.sendForeignNameQueryResponse(ctx, queryGUID) {
			return nil
		}
		s.forwardNameQueryToWorld(p)
		return nil
	}

	s.sendNameQueryResponse(ctx, queryGUID, charResp.Character.CharName, charResp.Character.CharRace, charResp.Character.CharClass, charResp.Character.CharGender)
	return nil
}

func (s *GameSession) sendForeignNameQueryResponse(ctx context.Context, queryGUID uint64) bool {
	if s.charServiceClient == nil {
		return false
	}

	realmID := realmIDFromClientPlayerGUID(queryGUID)
	if realmID == root.RealmID {
		return false
	}

	resp, err := s.charServiceClient.CharacterByGUID(ctx, &pbChar.CharacterByGUIDRequest{
		Api:           root.Ver,
		RealmID:       realmID,
		CharacterGUID: guid.PlayerLowGUID(queryGUID),
	})
	if err != nil || resp == nil || resp.Character == nil {
		return false
	}

	s.sendNameQueryResponse(ctx, queryGUID, resp.Character.CharName, resp.Character.CharRace, resp.Character.CharClass, resp.Character.CharGender)
	return true
}

func (s *GameSession) forwardNameQueryToWorld(p *packet.Packet) {
	if s.worldSocket != nil {
		s.worldSocket.SendPacket(p)
	}
}

func (s *GameSession) sendNameQueryResponse(ctx context.Context, queryGUID uint64, name string, race, class, gender uint32) {
	resp := packet.NewWriterWithSize(packet.SMsgNameQueryResponse, 0)
	resp.GUID(queryGUID)
	resp.Uint8(0)
	resp.String(name)

	if realmIDFromClientPlayerGUID(queryGUID) == root.RealmID {
		resp.Uint8(0)
	} else {
		realmName := "unknown realm"
		if s.realmNamesService != nil {
			if name, err := s.realmNamesService.NameByID(ctx, realmIDFromClientPlayerGUID(queryGUID)); err == nil {
				realmName = name
			}
		}
		resp.String(realmName)
	}

	resp.Uint8(uint8(race))
	resp.Uint8(uint8(gender))
	resp.Uint8(uint8(class))
	resp.Uint8(0)
	s.gameSocket.Send(resp)
}

func realmIDFromClientPlayerGUID(playerGUID uint64) uint32 {
	if playerGUID>>48 == 0 {
		if realmID := uint32((playerGUID >> 32) & 0xffff); realmID != 0 {
			return realmID
		}
	}
	if realmID := uint32(guid.New(playerGUID).GetRealmID()); realmID != 0 {
		return realmID
	}

	return root.RealmID
}
