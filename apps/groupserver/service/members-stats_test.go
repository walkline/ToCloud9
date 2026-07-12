package service

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/walkline/ToCloud9/shared/events"
)

func TestMembersStatsCollectorDropsPendingUpdatesOnLogout(t *testing.T) {
	lvl := uint8(4)
	collector := NewMembersStatsCollector(nil, nil, nil, 0)

	err := collector.HandleCharactersUpdates(events.GWEventCharactersUpdatesPayload{
		RealmID: 1,
		Updates: []*events.CharacterUpdate{{ID: 40554, Lvl: &lvl}},
	})
	assert.NoError(t, err)
	assert.Contains(t, collector.pending[1], uint64(40554))

	err = collector.HandleCharacterLoggedOut(events.GWEventCharacterLoggedOutPayload{
		RealmID:  1,
		CharGUID: 40554,
	})
	assert.NoError(t, err)
	assert.NotContains(t, collector.pending[1], uint64(40554))
}
