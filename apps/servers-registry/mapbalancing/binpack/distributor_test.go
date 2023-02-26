package binpack

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/walkline/ToCloud9/apps/servers-registry/repo"
)

func Test_knapsackBalancer_Distribute(t *testing.T) {
	tests := map[string]struct {
		weights MapsWeight
		servers []repo.GameServer
		want    []repo.GameServer
	}{
		"simple 2 servers 3 maps": {
			weights: map[uint32]uint32{1: 3, 0: 1, 3: 2},
			servers: []repo.GameServer{{}, {}},
			want:    []repo.GameServer{{AssignedMapsToHandle: []uint32{1}}, {AssignedMapsToHandle: []uint32{0, 3}}},
		},
		"simple 2 servers 3 maps and 1 server with exact map": {
			weights: map[uint32]uint32{1: 3, 0: 1, 3: 2},
			servers: []repo.GameServer{{}, {}, {AvailableMaps: []uint32{1}}},
			want: []repo.GameServer{
				{AvailableMaps: []uint32{1}, AssignedMapsToHandle: []uint32{1}},
				{AssignedMapsToHandle: []uint32{3}},
				{AssignedMapsToHandle: []uint32{0}},
			},
		},
		"simple 3 servers 3 maps": {
			weights: map[uint32]uint32{1: 3, 0: 1, 3: 2},
			servers: []repo.GameServer{{}, {}, {}},
			want:    []repo.GameServer{{AssignedMapsToHandle: []uint32{1}}, {AssignedMapsToHandle: []uint32{3}}, {AssignedMapsToHandle: []uint32{0}}},
		},
		"simple 1 server 3 maps": {
			weights: map[uint32]uint32{1: 3, 3: 2, 0: 1},
			servers: []repo.GameServer{{}},
			want:    []repo.GameServer{{AssignedMapsToHandle: []uint32{0, 1, 3}}},
		},
		// Hard to calculate expected output =P
		//"real example 2 servers": {
		//	weights: DefaultMapsWeight,
		//	servers: []repo.GameServer{{}, {}},
		//	want:    []repo.GameServer{},
		//},
		// Hard to calculate expected output =P
		//"real example 3 servers": {
		//	weights: DefaultMapsWeight,
		//	servers: []repo.GameServer{{}, {}, {}},
		//	want:    []repo.GameServer{},
		//},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			k := &binpackBalancer{
				weights: tt.weights,
			}
			r := k.Distribute(tt.servers)
			assert.ElementsMatch(t, tt.want, r)
		})
	}
}
