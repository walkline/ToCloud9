package perun

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRedrawBarrier_TryDraw(t *testing.T) {
	drawCounter := 0
	drawFunc := func() {
		drawCounter++
	}

	barrier := NewRedrawBarrier(10*time.Millisecond, drawFunc)

	started := time.Now()
	for i := 0; i < 30; i++ {
		barrier.TryDraw()
		time.Sleep(time.Millisecond)
	}

	assert.Equal(t, time.Since(started).Milliseconds()/10+1, int64(drawCounter))
}
