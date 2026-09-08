package session

import "testing"

func TestLocaleFromChallengeCountry(t *testing.T) {
	tests := []struct {
		name     string
		country  [4]byte
		expected uint8
	}{
		{
			name:     "french client",
			country:  [4]byte{'R', 'F', 'r', 'f'},
			expected: 2,
		},
		{
			name:     "english client",
			country:  [4]byte{'S', 'U', 'n', 'e'},
			expected: 0,
		},
		{
			name:     "russian client",
			country:  [4]byte{'U', 'R', 'u', 'r'},
			expected: 8,
		},
		{
			name:     "british client falls back to enUS",
			country:  [4]byte{'B', 'G', 'n', 'e'},
			expected: localeEnUS,
		},
		{
			name:     "garbage falls back to enUS",
			country:  [4]byte{0, 0, 0, 0},
			expected: localeEnUS,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if locale := localeFromChallengeCountry(test.country); locale != test.expected {
				t.Fatalf("unexpected locale, expected %d, got %d", test.expected, locale)
			}
		})
	}
}
