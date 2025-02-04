package guid

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewObjectGuidFromValues(t *testing.T) {
	guid := NewFromEntryAndCounter(Item, 512, 100)
	assert.Equal(t, Item, guid.GetHigh())
	assert.Equal(t, uint32(512), guid.GetEntry())
	assert.Equal(t, LowType(100), guid.GetCounter())
}

func TestCrossrealmPlayer(t *testing.T) {
	guid := NewCrossrealmPlayerGUID(42, 1000000095)
	assert.Equal(t, Player, guid.GetHigh())
	assert.Equal(t, uint16(42), guid.GetRealmID())
	assert.Equal(t, LowType(1000000095), guid.GetCounter())
}
