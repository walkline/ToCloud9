// Package gamedata provides static game data (class/race info, warrior spell
// IDs, base stats) used by the bot AI for making decisions without requiring
// runtime DBC parsing. This covers the WoW 3.3.5a patch (build 12340).
package gamedata

// Warrior spells by level range - the bot's primary class for testing.
// These are the baseline abilities a warrior would train while leveling.
var WarriorSpells = map[uint32]SpellInfo{
	// Melee combat abilities (used via CastSpell on target)
	78:   {Name: "Heroic Strike", Level: 1, RageCost: 15, Cooldown: 0, Range: 5, IsMelee: true},
	772:  {Name: "Rend", Level: 4, RageCost: 10, Cooldown: 0, Range: 5, IsMelee: true},
	6343: {Name: "Thunder Clap", Level: 6, RageCost: 20, Cooldown: 6, Range: 8, IsMelee: true},
	1715: {Name: "Hamstring", Level: 8, RageCost: 10, Cooldown: 0, Range: 5, IsMelee: true},
	7384: {Name: "Overpower", Level: 12, RageCost: 5, Cooldown: 5, Range: 5, IsMelee: true, IsProc: true}, // Needs dodge proc
	6572: {Name: "Revenge", Level: 14, RageCost: 5, Cooldown: 5, Range: 5, IsMelee: true, IsProc: true},   // Needs block/parry/dodge proc
	285:  {Name: "Heroic Strike (Rank 2)", Level: 16, RageCost: 15, Cooldown: 0, Range: 5, IsMelee: true},
	7386: {Name: "Sunder Armor", Level: 10, RageCost: 15, Cooldown: 0, Range: 5, IsMelee: true},
	845:  {Name: "Cleave", Level: 20, RageCost: 20, Cooldown: 0, Range: 5, IsMelee: true},
	6552: {Name: "Pummel", Level: 38, RageCost: 10, Cooldown: 10, Range: 5, IsMelee: true},
	1464: {Name: "Slam", Level: 30, RageCost: 15, Cooldown: 0, Range: 5, IsMelee: true},
	5308: {Name: "Execute", Level: 24, RageCost: 15, Cooldown: 0, Range: 5, IsMelee: true},
	1680: {Name: "Whirlwind", Level: 36, RageCost: 25, Cooldown: 10, Range: 8, IsMelee: true},

	// Shouts and buffs (not IsMelee - cast on self/area)
	2457:  {Name: "Battle Shout", Level: 1, RageCost: 10, Cooldown: 0, Range: 0, IsMelee: false},
	6673:  {Name: "Battle Shout (Rank 2)", Level: 12, RageCost: 10, Cooldown: 0, Range: 0, IsMelee: false},
	1160:  {Name: "Demoralizing Shout", Level: 14, RageCost: 10, Cooldown: 0, Range: 0, IsMelee: false},
	5246:  {Name: "Intimidating Shout", Level: 22, RageCost: 25, Cooldown: 120, Range: 0, IsMelee: false},
	18499: {Name: "Berserker Rage", Level: 32, RageCost: 0, Cooldown: 30, Range: 0, IsMelee: false},
	1719:  {Name: "Recklessness", Level: 50, RageCost: 0, Cooldown: 300, Range: 0, IsMelee: false},

	// Movement/utility abilities
	100:   {Name: "Charge", Level: 4, RageCost: 0, Cooldown: 15, Range: 25, IsMelee: false, MinRange: 8},
	355:   {Name: "Taunt", Level: 10, RageCost: 0, Cooldown: 8, Range: 30, IsMelee: false},
	2565:  {Name: "Shield Block", Level: 16, RageCost: 10, Cooldown: 60, Range: 0, IsMelee: false},
	12678: {Name: "Stance Mastery", Level: 20, RageCost: 0, Cooldown: 0, Range: 0, IsMelee: false},

	// Victory Rush is a proc (only after killing blow) - handled specially
	34428: {Name: "Victory Rush", Level: 6, RageCost: 0, Cooldown: 0, Range: 5, IsMelee: true, IsProc: true},
}

// SpellInfo holds static info about a spell.
type SpellInfo struct {
	Name     string
	Level    uint32
	ManaCost uint32
	RageCost uint32
	Cooldown float32 // seconds
	Range    float32 // yards
	MinRange float32 // yards
	IsMelee  bool
	IsHeal   bool
	IsProc   bool // requires a proc to be usable (e.g. Victory Rush)
}

// ClassSpells returns a simplified list of important spell IDs by class ID.
func ClassSpells(classID uint8) map[uint32]SpellInfo {
	switch classID {
	case 1: // Warrior
		return WarriorSpells
	default:
		return WarriorSpells // Fallback to warrior for now
	}
}

// GetSpellPriority returns a priority-ordered list of offensive spell IDs
// for the given class at the given level. Higher priority first.
// Excludes proc abilities, buffs, and non-combat spells.
func GetSpellPriority(classID uint8, level uint32) []uint32 {
	spells := ClassSpells(classID)
	type sp struct {
		id   uint32
		prio int
	}

	var available []sp
	for id, info := range spells {
		if info.Level > level || !info.IsMelee || info.IsProc {
			continue
		}
		prio := 50
		switch id {
		case 5308: // Execute (only when target < 20% HP)
			prio = 200
		case 7384: // Overpower
			prio = 150
		case 6572: // Revenge
			prio = 140
		case 772: // Rend (apply once)
			prio = 120
		case 7386: // Sunder Armor (stack 5 times)
			prio = 110
		case 6343: // Thunder Clap
			prio = 100
		case 285: // Heroic Strike Rank 2
			prio = 80
		case 78: // Heroic Strike
			prio = 70
		case 1715: // Hamstring
			prio = 30
		}
		available = append(available, sp{id: id, prio: prio})
	}

	// Sort by priority descending
	for i := 0; i < len(available); i++ {
		for j := i + 1; j < len(available); j++ {
			if available[j].prio > available[i].prio {
				available[i], available[j] = available[j], available[i]
			}
		}
	}

	result := make([]uint32, len(available))
	for i, s := range available {
		result[i] = s.id
	}
	return result
}

// RaceStartPosition returns the starting zone coordinates for a race.
func RaceStartPosition(race uint8) (mapID uint32, x, y, z float32) {
	switch race {
	case 1: // Human
		return 0, -8949.95, -132.493, 83.5312
	case 2: // Orc
		return 1, -618.518, -4251.67, 38.718
	case 3: // Dwarf
		return 0, -6240.32, 331.033, 382.758
	case 4: // Night Elf
		return 1, 10311.3, 832.463, 1326.41
	case 5: // Undead
		return 0, 1676.71, 1677.98, 121.67
	case 6: // Tauren
		return 1, -2917.58, -257.98, 52.9968
	case 7: // Gnome
		return 0, -6240.32, 331.033, 382.758
	case 8: // Troll
		return 1, -618.518, -4251.67, 38.718
	case 10: // Blood Elf - Eversong Woods / Sunstrider Isle
		return 530, 10349.6, -6357.29, 33.4026
	case 11: // Draenei - Azuremyst Isle / Ammen Vale
		return 530, -3961.64, -13931.2, 100.615
	default:
		return 0, -8949.95, -132.493, 83.5312
	}
}

// HoggerInfo contains known data about the Hogger NPC for testing.
var HoggerInfo = NPCInfo{
	Entry:    448,
	Name:     "Hogger",
	Level:    11,
	Health:   572,
	MapID:    0,
	PosX:     -10107.26,
	PosY:     617.83, // from creature table
	PosZ:     38.083,
	ZoneName: "Elwynn Forest",
}

// NPCInfo holds info about a known NPC.
type NPCInfo struct {
	Entry    uint32
	Name     string
	Level    uint32
	Health   uint32
	MapID    uint32
	PosX     float32
	PosY     float32
	PosZ     float32
	ZoneName string
}

// DungeonInfo holds info about a dungeon for group testing.
type DungeonInfo struct {
	Name          string
	MapID         uint32
	MinLevel      uint32
	MaxLevel      uint32
	EntranceMapID uint32
	EntranceX     float32
	EntranceY     float32
	EntranceZ     float32
	GroupSize     int
	BossEntries   []uint32
}

// Dungeons available for testing.
var Dungeons = map[string]DungeonInfo{
	"ragefire_chasm": {
		Name:          "Ragefire Chasm",
		MapID:         389,
		MinLevel:      15,
		MaxLevel:      21,
		EntranceMapID: 1,
		EntranceX:     1811.78,
		EntranceY:     -4410.5,
		EntranceZ:     -18.4704,
		GroupSize:     5,
		BossEntries:   []uint32{11517, 11518, 11519, 11520}, // Oggleflint, Taragaman, Jergosh, Bazzalan
	},
	"the_deadmines": {
		Name:          "The Deadmines",
		MapID:         36,
		MinLevel:      17,
		MaxLevel:      26,
		EntranceMapID: 0,
		EntranceX:     -11208.3,
		EntranceY:     1672.52,
		EntranceZ:     24.6361,
		GroupSize:     5,
		BossEntries:   []uint32{644, 642, 1763, 646, 639}, // Rhahk'Zor, Sneed, Gilnid, Mr.Smite, VanCleef
	},
}

// GrindZone represents a zone where bots can grind for XP.
type GrindZone struct {
	Name     string
	MapID    uint32
	CenterX  float32
	CenterY  float32
	CenterZ  float32
	Radius   float32
	MinLevel uint32
	MaxLevel uint32
}

// GrindZones is a list of zones for leveling.
var GrindZones = []GrindZone{
	// Undead starting area (Deathknell / Tirisfal Glades)
	{Name: "Deathknell", MapID: 0, CenterX: 1848, CenterY: 1608, CenterZ: 97, Radius: 100, MinLevel: 1, MaxLevel: 6},
	{Name: "Tirisfal Glades", MapID: 0, CenterX: 2259, CenterY: 312, CenterZ: 35, Radius: 200, MinLevel: 5, MaxLevel: 12},
	// Human starting area (Northshire / Elwynn Forest)
	{Name: "Northshire Valley", MapID: 0, CenterX: -8920, CenterY: -183, CenterZ: 81, Radius: 150, MinLevel: 1, MaxLevel: 6},
	{Name: "Elwynn Forest", MapID: 0, CenterX: -9465, CenterY: 75, CenterZ: 57, Radius: 300, MinLevel: 5, MaxLevel: 12},
	{Name: "Westfall", MapID: 0, CenterX: -10670, CenterY: 1033, CenterZ: 34, Radius: 400, MinLevel: 10, MaxLevel: 20},
	{Name: "Redridge Mountains", MapID: 0, CenterX: -9225, CenterY: -2200, CenterZ: 65, Radius: 300, MinLevel: 15, MaxLevel: 25},
	// Orc starting area (Durotar)
	{Name: "Valley of Trials", MapID: 1, CenterX: -618, CenterY: -4251, CenterZ: 38, Radius: 150, MinLevel: 1, MaxLevel: 6},
	{Name: "Durotar", MapID: 1, CenterX: 230, CenterY: -4738, CenterZ: 10, Radius: 300, MinLevel: 5, MaxLevel: 12},
	{Name: "The Barrens", MapID: 1, CenterX: -442, CenterY: -2650, CenterZ: 96, Radius: 500, MinLevel: 10, MaxLevel: 25},
}

// GetGrindZone returns a suitable grind zone for the given level and map.
func GetGrindZone(level uint32, mapID uint32) *GrindZone {
	var best *GrindZone
	for i := range GrindZones {
		zone := &GrindZones[i]
		if zone.MapID == mapID && level >= zone.MinLevel && level <= zone.MaxLevel {
			if best == nil || zone.MinLevel > best.MinLevel {
				best = zone
			}
		}
	}
	// Fallback: any matching level zone
	if best == nil {
		for i := range GrindZones {
			zone := &GrindZones[i]
			if level >= zone.MinLevel && level <= zone.MaxLevel {
				if best == nil || zone.MinLevel > best.MinLevel {
					best = zone
				}
			}
		}
	}
	return best
}
