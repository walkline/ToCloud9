package repo

import (
	"reflect"
	"testing"
)

func TestBattlegroundTemplate_GetAllBrackets(t1 *testing.T) {
	tests := map[string]struct {
		MinLvl uint8
		MaxLvl uint8

		want []uint8
	}{
		"all possible": {
			MinLvl: 1,
			MaxLvl: 80,
			want:   []uint8{1, 2, 3, 4, 5, 6, 7, 8},
		},
		"35-80": {
			MinLvl: 35,
			MaxLvl: 80,
			want:   []uint8{3, 4, 5, 6, 7, 8},
		},
		"35-80+": {
			MinLvl: 35,
			MaxLvl: 100,
			want:   []uint8{3, 4, 5, 6, 7, 8},
		},
	}
	for name, tt := range tests {
		t1.Run(name, func(t1 *testing.T) {
			t := &BattlegroundTemplate{
				MinLvl: tt.MinLvl,
				MaxLvl: tt.MaxLvl,
			}
			if got := t.GetAllBrackets(); !reflect.DeepEqual(got, tt.want) {
				t1.Errorf("GetAllBrackets() = %v, want %v", got, tt.want)
			}
		})
	}
}
