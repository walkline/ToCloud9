package slices

import "testing"

func TestSameBytes(t *testing.T) {
	tests := []struct {
		name string
		a    []byte
		b    []byte
		want bool
	}{
		{
			name: "diff size",
			a:    []byte{1, 2, 3},
			b:    []byte{1, 2},
			want: false,
		},
		{
			name: "diff content",
			a:    []byte{1, 2, 3},
			b:    []byte{1, 1, 3},
			want: false,
		},
		{
			name: "same slices",
			a:    []byte{1, 2, 3},
			b:    []byte{1, 2, 3},
			want: true,
		},
		{
			name: "empty",
			a:    []byte{},
			b:    []byte{},
			want: true,
		},
		{
			name: "nil a empty b",
			a:    nil,
			b:    []byte{},
			want: true,
		},
		{
			name: "nil b empty a",
			a:    []byte{},
			b:    nil,
			want: true,
		},
		{
			name: "nil a & b",
			a:    nil,
			b:    nil,
			want: true,
		},
		{
			name: "nil a not empty b",
			a:    nil,
			b:    []byte{1},
			want: false,
		},
		{
			name: "nil b not empty a",
			a:    []byte{1},
			b:    nil,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SameBytes(tt.a, tt.b); got != tt.want {
				t.Errorf("SameBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}
