package repo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWhoRequestFiltersByZone(t *testing.T) {
	r := NewCharactersOnlineInMem()

	addChar := func(guid uint64, zone uint32) {
		assert.NoError(t, r.Add(context.Background(), &Character{
			RealmID:   1,
			GatewayID: "gw1",
			CharGUID:  guid,
			CharName:  "Char",
			CharRace:  1,
			CharClass: 1,
			CharLevel: 5,
			CharZone:  zone,
		}))
	}
	addChar(1, 141)
	addChar(2, 215)

	baseQuery := CharactersWhoQuery{
		LvlMin:    1,
		LvlMax:    80,
		RaceMask:  ^uint32(0),
		ClassMask: ^uint32(0),
	}

	// Zone filter keeps only characters in the given zones.
	query := baseQuery
	query.Zones = []uint32{141}
	chars, err := r.WhoRequest(context.Background(), 1, 999, query)
	assert.NoError(t, err)
	assert.Len(t, chars, 1)
	assert.Equal(t, uint64(1), chars[0].CharGUID)

	// No zones in the query — no zone filtering.
	chars, err = r.WhoRequest(context.Background(), 1, 999, baseQuery)
	assert.NoError(t, err)
	assert.Len(t, chars, 2)

	// Zones without a single match filter everything out.
	query.Zones = []uint32{999}
	chars, err = r.WhoRequest(context.Background(), 1, 999, query)
	assert.NoError(t, err)
	assert.Len(t, chars, 0)
}
