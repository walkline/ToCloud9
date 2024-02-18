package slices

import (
	"reflect"
	"testing"
)

func TestReverseBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "Empty slice",
			input:    []byte{},
			expected: []byte{},
		},
		{
			name:     "Odd-length slice",
			input:    []byte{1, 2, 3, 4, 5},
			expected: []byte{5, 4, 3, 2, 1},
		},
		{
			name:     "Even-length slice",
			input:    []byte{6, 7, 8, 9, 10, 11},
			expected: []byte{11, 10, 9, 8, 7, 6},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ReverseBytes(test.input)
			if !reflect.DeepEqual(test.input, test.expected) {
				t.Errorf("got %v, want %v", test.input, test.expected)
			}
		})
	}
}
