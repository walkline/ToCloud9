package repo

import "testing"

func TestItemTemplateCache_MatchesFilters(t *testing.T) {
	cache := &ItemTemplateCache{
		templates: map[uint32]*ItemTemplate{
			// Consumable flask
			13510: {
				Entry:         13510,
				Class:         0, // Consumable
				SubClass:      1,
				Quality:       3, // Rare
				InventoryType: 0,
				ItemLevel:     60,
				Name:          "Flask of the Titans",
			},
			// Epic sword
			19364: {
				Entry:         19364,
				Class:         2, // Weapon
				SubClass:      7, // Sword
				Quality:       4, // Epic
				InventoryType: 13,
				ItemLevel:     75,
				Name:          "Ashkandi, Greatsword of the Brotherhood",
			},
			// Common gray item
			2589: {
				Entry:         2589,
				Class:         15, // Miscellaneous
				SubClass:      0,
				Quality:       0, // Poor/Gray
				InventoryType: 0,
				ItemLevel:     1,
				Name:          "Linen Cloth",
			},
			// Plate chest armor
			12640: {
				Entry:         12640,
				Class:         4, // Armor
				SubClass:      4, // Plate
				Quality:       4, // Epic
				InventoryType: 5, // Chest
				ItemLevel:     66,
				Name:          "Lionheart Helm",
			},
		},
	}

	tests := []struct {
		name          string
		itemEntry     uint32
		searchedName  string
		levelMin      uint32
		levelMax      uint32
		inventoryType uint32
		itemClass     uint32
		itemSubClass  uint32
		quality       uint32
		expected      bool
	}{
		{
			name:          "No filters - matches everything",
			itemEntry:     13510,
			searchedName:  "",
			levelMin:      0,
			levelMax:      0,
			inventoryType: 0xFFFFFFFF,
			itemClass:     0xFFFFFFFF,
			itemSubClass:  0xFFFFFFFF,
			quality:       0xFFFFFFFF,
			expected:      true,
		},
		{
			name:          "Filter by class - consumable matches",
			itemEntry:     13510,
			searchedName:  "",
			levelMin:      0,
			levelMax:      0,
			inventoryType: 0xFFFFFFFF,
			itemClass:     0,
			itemSubClass:  0xFFFFFFFF,
			quality:       0xFFFFFFFF,
			expected:      true,
		},
		{
			name:          "Filter by class - weapon doesn't match consumable",
			itemEntry:     13510,
			searchedName:  "",
			levelMin:      0,
			levelMax:      0,
			inventoryType: 0xFFFFFFFF,
			itemClass:     2, // Weapon
			itemSubClass:  0xFFFFFFFF,
			quality:       0xFFFFFFFF,
			expected:      false,
		},
		{
			name:          "Filter by quality - rare matches",
			itemEntry:     13510,
			searchedName:  "",
			levelMin:      0,
			levelMax:      0,
			inventoryType: 0xFFFFFFFF,
			itemClass:     0xFFFFFFFF,
			itemSubClass:  0xFFFFFFFF,
			quality:       3, // Rare
			expected:      true,
		},
		{
			name:          "Filter by quality - epic doesn't match rare",
			itemEntry:     13510,
			searchedName:  "",
			levelMin:      0,
			levelMax:      0,
			inventoryType: 0xFFFFFFFF,
			itemClass:     0xFFFFFFFF,
			itemSubClass:  0xFFFFFFFF,
			quality:       4, // Epic
			expected:      false,
		},
		{
			name:          "Filter by name - partial match",
			itemEntry:     13510,
			searchedName:  "flask",
			levelMin:      0,
			levelMax:      0,
			inventoryType: 0xFFFFFFFF,
			itemClass:     0xFFFFFFFF,
			itemSubClass:  0xFFFFFFFF,
			quality:       0xFFFFFFFF,
			expected:      true,
		},
		{
			name:          "Filter by name - case insensitive (caller lowercases)",
			itemEntry:     13510,
			searchedName:  "titans",
			levelMin:      0,
			levelMax:      0,
			inventoryType: 0xFFFFFFFF,
			itemClass:     0xFFFFFFFF,
			itemSubClass:  0xFFFFFFFF,
			quality:       0xFFFFFFFF,
			expected:      true,
		},
		{
			name:          "Filter by name - no match",
			itemEntry:     13510,
			searchedName:  "sword",
			levelMin:      0,
			levelMax:      0,
			inventoryType: 0xFFFFFFFF,
			itemClass:     0xFFFFFFFF,
			itemSubClass:  0xFFFFFFFF,
			quality:       0xFFFFFFFF,
			expected:      false,
		},
		{
			name:          "Filter by level range - within range",
			itemEntry:     13510,
			searchedName:  "",
			levelMin:      50,
			levelMax:      70,
			inventoryType: 0xFFFFFFFF,
			itemClass:     0xFFFFFFFF,
			itemSubClass:  0xFFFFFFFF,
			quality:       0xFFFFFFFF,
			expected:      true,
		},
		{
			name:          "Filter by level range - too low",
			itemEntry:     13510,
			searchedName:  "",
			levelMin:      70,
			levelMax:      80,
			inventoryType: 0xFFFFFFFF,
			itemClass:     0xFFFFFFFF,
			itemSubClass:  0xFFFFFFFF,
			quality:       0xFFFFFFFF,
			expected:      false,
		},
		{
			name:          "Filter by level range - too high",
			itemEntry:     13510,
			searchedName:  "",
			levelMin:      1,
			levelMax:      50,
			inventoryType: 0xFFFFFFFF,
			itemClass:     0xFFFFFFFF,
			itemSubClass:  0xFFFFFFFF,
			quality:       0xFFFFFFFF,
			expected:      false,
		},
		{
			name:          "Filter by inventory type - matches",
			itemEntry:     12640,
			searchedName:  "",
			levelMin:      0,
			levelMax:      0,
			inventoryType: 5, // Chest
			itemClass:     0xFFFFFFFF,
			itemSubClass:  0xFFFFFFFF,
			quality:       0xFFFFFFFF,
			expected:      true,
		},
		{
			name:          "Filter by inventory type - doesn't match",
			itemEntry:     12640,
			searchedName:  "",
			levelMin:      0,
			levelMax:      0,
			inventoryType: 1, // Head
			itemClass:     0xFFFFFFFF,
			itemSubClass:  0xFFFFFFFF,
			quality:       0xFFFFFFFF,
			expected:      false,
		},
		{
			name:          "Combined filters - all match",
			itemEntry:     19364,
			searchedName:  "ash",
			levelMin:      70,
			levelMax:      80,
			inventoryType: 13,
			itemClass:     2, // Weapon
			itemSubClass:  7, // Sword
			quality:       4, // Epic
			expected:      true,
		},
		{
			name:          "Combined filters - class doesn't match",
			itemEntry:     19364,
			searchedName:  "ash",
			levelMin:      70,
			levelMax:      80,
			inventoryType: 13,
			itemClass:     4, // Armor
			itemSubClass:  7,
			quality:       4,
			expected:      false,
		},
		{
			name:          "Combined filters - subclass doesn't match",
			itemEntry:     19364,
			searchedName:  "ash",
			levelMin:      70,
			levelMax:      80,
			inventoryType: 13,
			itemClass:     2,
			itemSubClass:  1, // Axe
			quality:       4,
			expected:      false,
		},
		{
			name:          "Gray item - quality 0",
			itemEntry:     2589,
			searchedName:  "",
			levelMin:      0,
			levelMax:      0,
			inventoryType: 0xFFFFFFFF,
			itemClass:     0xFFFFFFFF,
			itemSubClass:  0xFFFFFFFF,
			quality:       0, // Poor
			expected:      true,
		},
		{
			name:          "Consumable class 0 filter",
			itemEntry:     13510,
			searchedName:  "",
			levelMin:      0,
			levelMax:      0,
			inventoryType: 0xFFFFFFFF,
			itemClass:     0, // Consumable
			itemSubClass:  0xFFFFFFFF,
			quality:       0xFFFFFFFF,
			expected:      true,
		},
		{
			name:          "Non-existent item",
			itemEntry:     99999,
			searchedName:  "",
			levelMin:      0,
			levelMax:      0,
			inventoryType: 0xFFFFFFFF,
			itemClass:     0xFFFFFFFF,
			itemSubClass:  0xFFFFFFFF,
			quality:       0xFFFFFFFF,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cache.MatchesFilters(
				tt.itemEntry,
				tt.searchedName,
				tt.levelMin,
				tt.levelMax,
				tt.inventoryType,
				tt.itemClass,
				tt.itemSubClass,
				tt.quality,
			)

			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// Test edge cases for filter values
func TestItemTemplateCache_FilterEdgeCases(t *testing.T) {
	cache := &ItemTemplateCache{
		templates: map[uint32]*ItemTemplate{
			1: {
				Entry:         1,
				Class:         0,
				SubClass:      0,
				Quality:       0,
				InventoryType: 0,
				ItemLevel:     1,
				Name:          "Test Item",
			},
		},
	}

	// 0xFFFFFFFF means "no filter"
	if !cache.MatchesFilters(1, "", 0, 0, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF) {
		t.Error("0xFFFFFFFF should mean no filter, but item was filtered out")
	}

	// Zero values for class/quality should still filter
	if !cache.MatchesFilters(1, "", 0, 0, 0xFFFFFFFF, 0, 0xFFFFFFFF, 0xFFFFFFFF) {
		t.Error("Class 0 filter should match class 0 item")
	}

	if !cache.MatchesFilters(1, "", 0, 0, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0) {
		t.Error("Quality 0 filter should match quality 0 item")
	}

	// Non-matching zero value should filter out
	cache.templates[1].Class = 1
	if cache.MatchesFilters(1, "", 0, 0, 0xFFFFFFFF, 0, 0xFFFFFFFF, 0xFFFFFFFF) {
		t.Error("Class 0 filter should not match class 1 item")
	}
}

// Test level range filtering specifically
func TestItemTemplateCache_LevelRangeFiltering(t *testing.T) {
	cache := &ItemTemplateCache{
		templates: map[uint32]*ItemTemplate{
			1: {Entry: 1, ItemLevel: 50, Name: "Level 50 Item"},
			2: {Entry: 2, ItemLevel: 1, Name: "Level 1 Item"},
			3: {Entry: 3, ItemLevel: 80, Name: "Level 80 Item"},
		},
	}

	tests := []struct {
		name      string
		itemEntry uint32
		levelMin  uint32
		levelMax  uint32
		expected  bool
	}{
		{"No level filter", 1, 0, 0, true},
		{"Exact min boundary", 1, 50, 60, true},
		{"Exact max boundary", 1, 40, 50, true},
		{"Below min", 1, 51, 60, false},
		{"Above max", 1, 40, 49, false},
		{"Level 1 with min 1", 2, 1, 10, true},
		{"Level 80 with max 80", 3, 70, 80, true},
		{"Wide range", 1, 1, 100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cache.MatchesFilters(
				tt.itemEntry,
				"",
				tt.levelMin,
				tt.levelMax,
				0xFFFFFFFF,
				0xFFFFFFFF,
				0xFFFFFFFF,
				0xFFFFFFFF,
			)

			if result != tt.expected {
				t.Errorf("Item level %d with range [%d, %d]: expected %v, got %v",
					cache.templates[tt.itemEntry].ItemLevel,
					tt.levelMin,
					tt.levelMax,
					tt.expected,
					result)
			}
		})
	}
}

// Test name search edge cases
func TestItemTemplateCache_NameSearching(t *testing.T) {
	cache := &ItemTemplateCache{
		templates: map[uint32]*ItemTemplate{
			1: {Entry: 1, Name: "Thunderfury, Blessed Blade of the Windseeker"},
			2: {Entry: 2, Name: "Atiesh, Greatstaff of the Guardian"},
			3: {Entry: 3, Name: "Sulfuras, Hand of Ragnaros"},
		},
	}

	tests := []struct {
		name         string
		itemEntry    uint32
		searchedName string
		expected     bool
	}{
		{"Empty search matches all", 1, "", true},
		{"Partial name - beginning", 1, "thunder", true},
		{"Partial name - middle", 1, "blessed", true},
		{"Partial name - end", 1, "windseeker", true},
		{"Lowercased search (caller's responsibility)", 1, "thunderfury", true},
		{"Lowercased partial", 1, "blessed", true},
		{"Punctuation in search", 1, "thunderfury,", true},
		{"No match", 1, "sulfuras", false},
		{"Substring too short", 2, "ati", true},
		{"Single character", 3, "s", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cache.MatchesFilters(
				tt.itemEntry,
				tt.searchedName,
				0,
				0,
				0xFFFFFFFF,
				0xFFFFFFFF,
				0xFFFFFFFF,
				0xFFFFFFFF,
			)

			if result != tt.expected {
				t.Errorf("Search '%s' in '%s': expected %v, got %v",
					tt.searchedName,
					cache.templates[tt.itemEntry].Name,
					tt.expected,
					result)
			}
		})
	}
}
