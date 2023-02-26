package session

import (
	"context"
	"fmt"

	root "github.com/walkline/ToCloud9/apps/game-load-balancer"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/packet"
	"github.com/walkline/ToCloud9/gen/characters/pb"
)

func (s *GameSession) HandleWho(ctx context.Context, p *packet.Packet) error {
	s.logger.Debug().Msg("Handling who")

	r := p.Reader()
	lvlMin := r.Uint32()
	lvlMax := r.Uint32()

	playerName := r.String()
	guildName := r.String()

	raceMask := r.Uint32()
	classMask := r.Uint32()

	zonesCount := r.Uint32()
	if zonesCount > 10 {
		return fmt.Errorf("zoneCount is invalid - %d, should be <= 10", zonesCount)
	}

	zones := make([]uint32, zonesCount)
	for i := uint32(0); i < zonesCount; i++ {
		zones[i] = r.Uint32()
	}

	strCount := r.Uint32()
	if strCount > 4 {
		return fmt.Errorf("zoneCount is invalid - %d, should be <= 4", zonesCount)
	}

	strs := make([]string, strCount)
	for i := uint32(0); i < strCount; i++ {
		strs[i] = r.String()
	}

	resp, err := s.charServiceClient.WhoQuery(ctx, &pb.WhoQueryRequest{
		Api:           root.Ver,
		CharacterGUID: s.character.GUID,
		RealmID:       root.RealmID,
		LvlMin:        lvlMin,
		LvlMax:        lvlMax,
		PlayerName:    playerName,
		GuildName:     guildName,
		RaceMask:      raceMask,
		ClassMask:     classMask,
		Strings:       strs,
	})
	if err != nil {
		return err
	}

	w := packet.NewWriter(packet.SMsgWho)
	w.Uint32(uint32(len(resp.ItemsToDisplay)))
	w.Uint32(resp.TotalFound)
	for _, item := range resp.ItemsToDisplay {
		w.String(item.Name)
		w.String(item.Guild)
		w.Uint32(item.Lvl)
		w.Uint32(item.Class)
		w.Uint32(item.Race)
		w.Uint8(uint8(item.Race))
		w.Uint32(item.ZoneID)
	}

	s.gameSocket.Send(w)

	return nil
}
