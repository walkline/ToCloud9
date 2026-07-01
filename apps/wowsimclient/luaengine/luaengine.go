// Package luaengine provides a Lua scripting runtime for the WoW bot,
// using github.com/Shopify/go-lua. It exposes bot actions and queries to
// Lua scripts and allows runtime behavior modification.
package luaengine

import (
	"fmt"
	"sync"

	lua "github.com/Shopify/go-lua"

	"github.com/walkline/ToCloud9/apps/wowsimclient/behaviortree"
)

// BotAPI is the interface that the Lua engine uses to interact with the bot.
type BotAPI interface {
	// Movement
	GetPosition() (x, y, z, o float32)
	MoveTo(x, y, z float32) error
	StopMoving() error

	// Combat
	AttackTarget(guid uint64) error
	StopAttack() error
	SetTarget(guid uint64) error
	CastSpell(spellID uint32, targetGUID uint64) error
	IsSpellReady(spellID uint32) bool
	GetHealth() (current, max uint32)
	GetPower() (current, max uint32)
	GetLevel() uint32
	InCombat() bool
	IsAlive() bool
	GetTargetGUID() uint64

	// Objects
	GetNearbyUnits(maxDist float32) []UnitInfo
	GetNearbyPlayers(maxDist float32) []UnitInfo
	GetUnitInfo(guid uint64) *UnitInfo

	// Actions
	SendChat(message string) error
	SendCommand(command string) error
	Loot(guid uint64) error
	LootAll(guid uint64) error

	// Logging
	Log(format string, args ...interface{})
}

// UnitInfo is a simplified view of a nearby unit passed to Lua.
type UnitInfo struct {
	GUID      uint64
	Entry     uint32
	Health    uint32
	MaxHealth uint32
	Level     uint32
	PosX      float32
	PosY      float32
	PosZ      float32
	IsAlive   bool
	IsPlayer  bool
	Distance  float32
	Name      string
}

// Engine wraps a Lua state and provides methods to load/run scripts.
type Engine struct {
	mu      sync.Mutex
	state   *lua.State
	bot     BotAPI
	tree    *behaviortree.Tree
	actions map[string]func(bb *behaviortree.Blackboard) behaviortree.Status

	// Script-defined tick function name
	tickFunc string
}

// NewEngine creates a new Lua engine bound to the given bot API.
func NewEngine(bot BotAPI) *Engine {
	e := &Engine{
		bot:      bot,
		actions:  make(map[string]func(bb *behaviortree.Blackboard) behaviortree.Status),
		tickFunc: "on_tick",
	}
	e.initState()
	return e
}

func (e *Engine) initState() {
	e.state = lua.NewState()
	lua.OpenLibraries(e.state)

	// Register bot API functions
	e.registerBotFunctions()
}

func (e *Engine) registerBotFunctions() {
	L := e.state

	// bot table
	L.NewTable()

	// bot.log(msg)
	e.setFunc("log", func(l *lua.State) int {
		msg, _ := l.ToString(1)
		e.bot.Log("[Lua] %s", msg)
		return 0
	})

	// bot.get_position() -> x, y, z, o
	e.setFunc("get_position", func(l *lua.State) int {
		x, y, z, o := e.bot.GetPosition()
		l.PushNumber(float64(x))
		l.PushNumber(float64(y))
		l.PushNumber(float64(z))
		l.PushNumber(float64(o))
		return 4
	})

	// bot.move_to(x, y, z) -> bool
	e.setFunc("move_to", func(l *lua.State) int {
		x, _ := l.ToNumber(1)
		y, _ := l.ToNumber(2)
		z, _ := l.ToNumber(3)
		err := e.bot.MoveTo(float32(x), float32(y), float32(z))
		l.PushBoolean(err == nil)
		return 1
	})

	// bot.stop_moving()
	e.setFunc("stop_moving", func(l *lua.State) int {
		e.bot.StopMoving()
		return 0
	})

	// bot.attack(guid)
	e.setFunc("attack", func(l *lua.State) int {
		guid, _ := l.ToNumber(1)
		err := e.bot.AttackTarget(uint64(guid))
		l.PushBoolean(err == nil)
		return 1
	})

	// bot.stop_attack()
	e.setFunc("stop_attack", func(l *lua.State) int {
		e.bot.StopAttack()
		return 0
	})

	// bot.set_target(guid)
	e.setFunc("set_target", func(l *lua.State) int {
		guid, _ := l.ToNumber(1)
		e.bot.SetTarget(uint64(guid))
		return 0
	})

	// bot.cast_spell(spellID, targetGUID) -> bool
	e.setFunc("cast_spell", func(l *lua.State) int {
		spellID, _ := l.ToNumber(1)
		targetGUID, _ := l.ToNumber(2)
		err := e.bot.CastSpell(uint32(spellID), uint64(targetGUID))
		l.PushBoolean(err == nil)
		return 1
	})

	// bot.is_spell_ready(spellID) -> bool
	e.setFunc("is_spell_ready", func(l *lua.State) int {
		spellID, _ := l.ToNumber(1)
		ready := e.bot.IsSpellReady(uint32(spellID))
		l.PushBoolean(ready)
		return 1
	})

	// bot.get_health() -> current, max
	e.setFunc("get_health", func(l *lua.State) int {
		cur, max := e.bot.GetHealth()
		l.PushNumber(float64(cur))
		l.PushNumber(float64(max))
		return 2
	})

	// bot.get_power() -> current, max
	e.setFunc("get_power", func(l *lua.State) int {
		cur, max := e.bot.GetPower()
		l.PushNumber(float64(cur))
		l.PushNumber(float64(max))
		return 2
	})

	// bot.get_level() -> number
	e.setFunc("get_level", func(l *lua.State) int {
		l.PushNumber(float64(e.bot.GetLevel()))
		return 1
	})

	// bot.in_combat() -> bool
	e.setFunc("in_combat", func(l *lua.State) int {
		l.PushBoolean(e.bot.InCombat())
		return 1
	})

	// bot.is_alive() -> bool
	e.setFunc("is_alive", func(l *lua.State) int {
		l.PushBoolean(e.bot.IsAlive())
		return 1
	})

	// bot.get_target() -> guid
	e.setFunc("get_target", func(l *lua.State) int {
		l.PushNumber(float64(e.bot.GetTargetGUID()))
		return 1
	})

	// bot.get_nearby_units(maxDist) -> table of units
	e.setFunc("get_nearby_units", func(l *lua.State) int {
		dist, _ := l.ToNumber(1)
		if dist <= 0 {
			dist = 30
		}
		units := e.bot.GetNearbyUnits(float32(dist))
		l.NewTable()
		for i, u := range units {
			l.PushNumber(float64(i + 1))
			pushUnitInfo(l, &u)
			l.SetTable(-3)
		}
		return 1
	})

	// bot.get_nearby_players(maxDist) -> table of units
	e.setFunc("get_nearby_players", func(l *lua.State) int {
		dist, _ := l.ToNumber(1)
		if dist <= 0 {
			dist = 30
		}
		players := e.bot.GetNearbyPlayers(float32(dist))
		l.NewTable()
		for i, u := range players {
			l.PushNumber(float64(i + 1))
			pushUnitInfo(l, &u)
			l.SetTable(-3)
		}
		return 1
	})

	// bot.get_unit(guid) -> unit table or nil
	e.setFunc("get_unit", func(l *lua.State) int {
		guid, _ := l.ToNumber(1)
		info := e.bot.GetUnitInfo(uint64(guid))
		if info == nil {
			l.PushNil()
		} else {
			pushUnitInfo(l, info)
		}
		return 1
	})

	// bot.send_chat(message)
	e.setFunc("send_chat", func(l *lua.State) int {
		msg, _ := l.ToString(1)
		e.bot.SendChat(msg)
		return 0
	})

	// bot.send_command(command)
	e.setFunc("send_command", func(l *lua.State) int {
		cmd, _ := l.ToString(1)
		e.bot.SendCommand(cmd)
		return 0
	})

	// bot.loot(guid)
	e.setFunc("loot", func(l *lua.State) int {
		guid, _ := l.ToNumber(1)
		e.bot.Loot(uint64(guid))
		return 0
	})

	// bot.loot_all(guid)
	e.setFunc("loot_all", func(l *lua.State) int {
		guid, _ := l.ToNumber(1)
		e.bot.LootAll(uint64(guid))
		return 0
	})

	L.SetGlobal("bot")
}

func (e *Engine) setFunc(name string, fn lua.Function) {
	e.state.PushGoFunction(fn)
	e.state.SetField(-2, name)
}

func pushUnitInfo(l *lua.State, u *UnitInfo) {
	l.NewTable()

	l.PushNumber(float64(u.GUID))
	l.SetField(-2, "guid")

	l.PushNumber(float64(u.Entry))
	l.SetField(-2, "entry")

	l.PushNumber(float64(u.Health))
	l.SetField(-2, "health")

	l.PushNumber(float64(u.MaxHealth))
	l.SetField(-2, "max_health")

	l.PushNumber(float64(u.Level))
	l.SetField(-2, "level")

	l.PushNumber(float64(u.PosX))
	l.SetField(-2, "x")

	l.PushNumber(float64(u.PosY))
	l.SetField(-2, "y")

	l.PushNumber(float64(u.PosZ))
	l.SetField(-2, "z")

	l.PushBoolean(u.IsAlive)
	l.SetField(-2, "is_alive")

	l.PushBoolean(u.IsPlayer)
	l.SetField(-2, "is_player")

	l.PushNumber(float64(u.Distance))
	l.SetField(-2, "distance")

	l.PushString(u.Name)
	l.SetField(-2, "name")
}

// DoString executes Lua code.
func (e *Engine) DoString(code string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return lua.DoString(e.state, code)
}

// DoFile loads and executes a Lua file.
func (e *Engine) DoFile(path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := lua.LoadFile(e.state, path, ""); err != nil {
		return fmt.Errorf("load lua file %s: %w", path, err)
	}
	return e.state.ProtectedCall(0, lua.MultipleReturns, 0)
}

// CallTick calls the global on_tick function if it exists.
// Returns true if the function was found and called.
func (e *Engine) CallTick() bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.state.Global(e.tickFunc)
	if !e.state.IsFunction(-1) {
		e.state.Pop(1)
		return false
	}
	if err := e.state.ProtectedCall(0, 0, 0); err != nil {
		e.bot.Log("[Lua] tick error: %v", err)
	}
	return true
}

// CallFunction calls a named global Lua function with no arguments.
func (e *Engine) CallFunction(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.state.Global(name)
	if !e.state.IsFunction(-1) {
		e.state.Pop(1)
		return fmt.Errorf("lua function %q not found", name)
	}
	return e.state.ProtectedCall(0, 0, 0)
}

// Reload reinitializes the Lua state and reloads a script file.
func (e *Engine) Reload(scriptPath string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.initState()
	if scriptPath != "" {
		if err := lua.LoadFile(e.state, scriptPath, ""); err != nil {
			return err
		}
		return e.state.ProtectedCall(0, lua.MultipleReturns, 0)
	}
	return nil
}

// SetTickFunc sets the name of the Lua function to call on each tick.
func (e *Engine) SetTickFunc(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.tickFunc = name
}

// Close releases the Lua state.
func (e *Engine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state = nil
}
