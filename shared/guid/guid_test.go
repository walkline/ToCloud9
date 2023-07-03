package guid

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewObjectGuidFromValues(t *testing.T) {
	guid := NewObjectGuidFromValues(Item, 512, 100)
	assert.Equal(t, Item, guid.GetHigh())
	assert.Equal(t, uint32(512), guid.GetEntry())
	assert.Equal(t, LowType(100), guid.GetCounter())
}
