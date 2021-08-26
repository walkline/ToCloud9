package slices

import (
	"reflect"
	"testing"
)

func TestAppendBytes(t *testing.T) {
	tests := []struct {
		name string
		args [][]byte
		want []byte
	}{
		{
			name: "combine 3 slices",
			args: [][]byte{{1, 2, 3, 4}, {5, 6, 7}, {8, 9}},
			want: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9},
		},
		{
			name: "using empty slice",
			args: [][]byte{},
			want: []byte{},
		},
		{
			name: "using nil",
			args: nil,
			want: []byte{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AppendBytes(tt.args...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AppendBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}
