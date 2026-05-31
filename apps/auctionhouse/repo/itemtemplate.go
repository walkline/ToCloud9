package repo

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
)

// ItemTemplate contains item template data needed for auction filtering
type ItemTemplate struct {
	Entry         uint32
	Class         uint32
	SubClass      uint32
	Quality       uint32
	InventoryType uint32
	ItemLevel     uint32
	RequiredLevel uint32
	Name          string
}

// ItemTemplateCache caches item templates for filtering
type ItemTemplateCache struct {
	mu        sync.RWMutex
	templates map[uint32]*ItemTemplate
}

// NewItemTemplateCache loads item templates from world database
func NewItemTemplateCache(worldDB *sql.DB) (*ItemTemplateCache, error) {
	query := `SELECT entry, class, subclass, Quality, InventoryType, ItemLevel, RequiredLevel, name
	          FROM item_template`

	rows, err := worldDB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to load item templates: %w", err)
	}
	defer rows.Close()

	templates := make(map[uint32]*ItemTemplate)
	count := 0

	for rows.Next() {
		var tmpl ItemTemplate
		err := rows.Scan(
			&tmpl.Entry,
			&tmpl.Class,
			&tmpl.SubClass,
			&tmpl.Quality,
			&tmpl.InventoryType,
			&tmpl.ItemLevel,
			&tmpl.RequiredLevel,
			&tmpl.Name,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan item template: %w", err)
		}

		templates[tmpl.Entry] = &tmpl
		count++
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating item templates: %w", err)
	}

	log.Info().Int("count", count).Msg("Loaded item templates into cache")

	return &ItemTemplateCache{
		templates: templates,
	}, nil
}

// Get returns an item template by entry ID
func (c *ItemTemplateCache) Get(entry uint32) *ItemTemplate {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.templates[entry]
}

// MatchesFilters checks if an item matches the given auction search filters
func (c *ItemTemplateCache) MatchesFilters(
	entry uint32,
	searchedName string,
	levelMin, levelMax, inventoryType, itemClass, itemSubClass, quality uint32,
) bool {
	tmpl := c.Get(entry)
	if tmpl == nil {
		return false
	}

	// Name filter
	if searchedName != "" {
		nameLower := strings.ToLower(tmpl.Name)
		if !strings.Contains(nameLower, searchedName) {
			return false
		}
	}

	// Level filters
	if levelMin > 0 && tmpl.ItemLevel < levelMin {
		return false
	}
	if levelMax > 0 && tmpl.ItemLevel > levelMax {
		return false
	}

	// Inventory type filter (0xFFFFFFFF means no filter)
	if inventoryType != 0xFFFFFFFF && tmpl.InventoryType != inventoryType {
		return false
	}

	// Class filter (0xFFFFFFFF means no filter)
	if itemClass != 0xFFFFFFFF && tmpl.Class != itemClass {
		return false
	}

	// SubClass filter (0xFFFFFFFF means no filter)
	if itemSubClass != 0xFFFFFFFF && tmpl.SubClass != itemSubClass {
		return false
	}

	// Quality filter (0xFFFFFFFF means no filter)
	if quality != 0xFFFFFFFF && tmpl.Quality != quality {
		return false
	}

	return true
}

// MatchesUsableFilter checks if item is usable by player (simplified - can be expanded)
func (c *ItemTemplateCache) MatchesUsableFilter(entry uint32, playerLevel uint32) bool {
	tmpl := c.Get(entry)
	if tmpl == nil {
		return false
	}

	// Basic check: player level >= required level
	return playerLevel >= tmpl.RequiredLevel
}
