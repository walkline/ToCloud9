package guids

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type diapasonsProviderMock struct {
	diapasonSize uint64
	counter      uint64

	diapasonsToReturn int

	delay time.Duration
}

func (d *diapasonsProviderMock) NewDiapasons(ctx context.Context) ([]diapason, error) {
	if d.delay != 0 {
		time.Sleep(d.delay)
	}

	res := []diapason{}
	for i := 0; i < d.diapasonsToReturn; i++ {
		diap := diapason{start: d.counter + 1}
		d.counter += d.diapasonSize
		diap.end = d.counter
		res = append(res, diap)
	}
	return res, nil
}

func Test_threadUnsafeGuidProvider_Next_HappyPath(t *testing.T) {
	diapasonsProvider := &diapasonsProviderMock{
		diapasonSize:      10,
		diapasonsToReturn: 1,
		delay:             time.Millisecond,
	}

	provider, err := NewThreadUnsafeGuidProvider(context.Background(), diapasonsProvider, 60)
	assert.NoError(t, err)

	for i := 0; i < 100; i++ {
		assert.Equal(t, uint64(i+1), provider.Next())
	}
}

func Test_threadUnsafeGuidProvider_Next_SmallestDiapasons(t *testing.T) {
	diapasonsProvider := &diapasonsProviderMock{
		diapasonSize:      1,
		diapasonsToReturn: 1,
		delay:             time.Millisecond,
	}

	provider, err := NewThreadUnsafeGuidProvider(context.Background(), diapasonsProvider, 60)
	assert.NoError(t, err)

	for i := 0; i < 100; i++ {
		assert.Equal(t, uint64(i+1), provider.Next())
	}
}

func Test_threadUnsafeGuidProvider_Next_DiapasonSize2(t *testing.T) {
	diapasonsProvider := &diapasonsProviderMock{
		diapasonSize:      2,
		diapasonsToReturn: 1,
		delay:             time.Millisecond,
	}

	provider, err := NewThreadUnsafeGuidProvider(context.Background(), diapasonsProvider, 60)
	assert.NoError(t, err)

	for i := 0; i < 100; i++ {
		assert.Equal(t, uint64(i+1), provider.Next())
	}
}

func Test_threadUnsafeGuidProvider_Next_TwoDiapasons(t *testing.T) {
	diapasonsProvider := &diapasonsProviderMock{
		diapasonSize:      10,
		diapasonsToReturn: 2,
		delay:             time.Millisecond,
	}

	provider, err := NewThreadUnsafeGuidProvider(context.Background(), diapasonsProvider, 60)
	assert.NoError(t, err)

	for i := 0; i < 100; i++ {
		assert.Equal(t, uint64(i+1), provider.Next())
	}
}
