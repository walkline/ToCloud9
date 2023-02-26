package service

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAvailableDiapasons_UseGuids(t *testing.T) {
	tests := map[string]struct {
		diapasons       []GuidDiapasonWithState
		amountToRequest uint64
		exp             []GuidDiapason
	}{
		"one diapason fulfill": {
			diapasons: []GuidDiapasonWithState{
				{
					GuidDiapason: GuidDiapason{
						Start: 1000,
						End:   2000,
					},
					CurrentGuid: 1500,
				},
			},
			amountToRequest: 40,
			exp: []GuidDiapason{
				{
					Start: 1500,
					End:   1539,
				},
			},
		},
		"one diapason drain": {
			diapasons: []GuidDiapasonWithState{
				{
					GuidDiapason: GuidDiapason{
						Start: 1000,
						End:   2000,
					},
					CurrentGuid: 1500,
				},
			},
			amountToRequest: 4000,
			exp: []GuidDiapason{
				{
					Start: 1500,
					End:   2000,
				},
			},
		},
		"several diapasons fulfill edge": {
			diapasons: []GuidDiapasonWithState{
				{
					GuidDiapason: GuidDiapason{
						Start: 1000,
						End:   2000,
					},
					CurrentGuid: 1500,
				},
				{
					GuidDiapason: GuidDiapason{
						Start: 5000,
						End:   6000,
					},
					CurrentGuid: 5500,
				},
			},
			amountToRequest: 1000,
			exp: []GuidDiapason{
				{
					Start: 1500,
					End:   2000,
				},
				{
					Start: 5500,
					End:   5998,
				},
			},
		},
		"several diapasons fulfill with leftovers": {
			diapasons: []GuidDiapasonWithState{
				{
					GuidDiapason: GuidDiapason{
						Start: 1000,
						End:   2000,
					},
					CurrentGuid: 1500,
				},
				{
					GuidDiapason: GuidDiapason{
						Start: 5000,
						End:   6000,
					},
					CurrentGuid: 5500,
				},
			},
			amountToRequest: 600,
			exp: []GuidDiapason{
				{
					Start: 1500,
					End:   2000,
				},
				{
					Start: 5500,
					End:   5598,
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			d := &AvailableDiapasons{
				Diapasons: tt.diapasons,
			}
			if got := d.UseGuids(tt.amountToRequest); !reflect.DeepEqual(got, tt.exp) {
				t.Errorf("UseGuids() = %v, want %v", got, tt.exp)
			}
		})
	}
}

func TestAvailableDiapasons_CleanupEmpty(t *testing.T) {
	tests := map[string]struct {
		diapasons    []GuidDiapasonWithState
		expDiapasons []GuidDiapasonWithState
	}{
		"full cleanup": {
			diapasons: []GuidDiapasonWithState{
				{
					GuidDiapason: GuidDiapason{End: 5},
					CurrentGuid:  6,
				},
				{
					GuidDiapason: GuidDiapason{End: 6},
					CurrentGuid:  7,
				},
				{
					GuidDiapason: GuidDiapason{End: 7},
					CurrentGuid:  8,
				},
			},
			expDiapasons: []GuidDiapasonWithState{},
		},
		"partly cleanup": {
			diapasons: []GuidDiapasonWithState{
				{
					GuidDiapason: GuidDiapason{End: 5},
					CurrentGuid:  6,
				},
				{
					GuidDiapason: GuidDiapason{End: 6},
					CurrentGuid:  7,
				},
				{
					GuidDiapason: GuidDiapason{End: 7},
					CurrentGuid:  7,
				},
			},
			expDiapasons: []GuidDiapasonWithState{
				{
					GuidDiapason: GuidDiapason{End: 7},
					CurrentGuid:  7,
				},
			},
		},
		"untouched": {
			diapasons: []GuidDiapasonWithState{
				{
					GuidDiapason: GuidDiapason{End: 5},
					CurrentGuid:  4,
				},
				{
					GuidDiapason: GuidDiapason{End: 6},
					CurrentGuid:  5,
				},
				{
					GuidDiapason: GuidDiapason{End: 7},
					CurrentGuid:  6,
				},
			},
			expDiapasons: []GuidDiapasonWithState{
				{
					GuidDiapason: GuidDiapason{End: 5},
					CurrentGuid:  4,
				},
				{
					GuidDiapason: GuidDiapason{End: 6},
					CurrentGuid:  5,
				},
				{
					GuidDiapason: GuidDiapason{End: 7},
					CurrentGuid:  6,
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			d := &AvailableDiapasons{
				Diapasons: tt.diapasons,
			}
			d.CleanupEmpty()
			assert.Equal(t, tt.expDiapasons, d.Diapasons)
		})
	}
}
