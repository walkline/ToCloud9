package events

import (
	"reflect"
	"testing"
)

func TestGameServer_OnlyNewMaps(t *testing.T) {
	tests := map[string]struct {
		OldAssignedMapsToHandle []uint32
		NewAssignedMapsToHandle []uint32
		Exp                     []uint32
	}{
		"same": {
			OldAssignedMapsToHandle: []uint32{1, 2, 3},
			NewAssignedMapsToHandle: []uint32{1, 2, 3},
			Exp:                     []uint32{},
		},
		"new": {
			OldAssignedMapsToHandle: []uint32{1, 2, 3},
			NewAssignedMapsToHandle: []uint32{1, 2, 3, 4, 5},
			Exp:                     []uint32{4, 5},
		},
		"removed": {
			OldAssignedMapsToHandle: []uint32{1, 2, 3},
			NewAssignedMapsToHandle: []uint32{1, 2},
			Exp:                     []uint32{},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := GameServer{
				OldAssignedMapsToHandle: tt.OldAssignedMapsToHandle,
				NewAssignedMapsToHandle: tt.NewAssignedMapsToHandle,
			}
			if got := s.OnlyNewMaps(); !reflect.DeepEqual(got, tt.Exp) {
				t.Errorf("OnlyNewMaps() = %v, want %v", got, tt.Exp)
			}
		})
	}
}

func TestGameServer_OnlyRemovedMaps(t *testing.T) {
	tests := map[string]struct {
		OldAssignedMapsToHandle []uint32
		NewAssignedMapsToHandle []uint32
		Exp                     []uint32
	}{
		"same": {
			OldAssignedMapsToHandle: []uint32{1, 2, 3},
			NewAssignedMapsToHandle: []uint32{1, 2, 3},
			Exp:                     []uint32{},
		},
		"new": {
			OldAssignedMapsToHandle: []uint32{1, 2, 3},
			NewAssignedMapsToHandle: []uint32{1, 2, 3, 4, 5},
			Exp:                     []uint32{},
		},
		"removed": {
			OldAssignedMapsToHandle: []uint32{1, 2, 3},
			NewAssignedMapsToHandle: []uint32{1, 2},
			Exp:                     []uint32{3},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := GameServer{
				OldAssignedMapsToHandle: tt.OldAssignedMapsToHandle,
				NewAssignedMapsToHandle: tt.NewAssignedMapsToHandle,
			}
			if got := s.OnlyRemovedMaps(); !reflect.DeepEqual(got, tt.Exp) {
				t.Errorf("OnlyRemovedMaps() = %v, want %v", got, tt.Exp)
			}
		})
	}
}
