package wowsimclient

import (
	"math"
	"time"

	"github.com/walkline/ToCloud9/apps/wowsimclient/navigation"
)

// MovementSender is the interface used by MovementController to emit movement packets.
// This decouples the movement logic from the concrete WorldClient.
type MovementSender interface {
	MoveStartForward(x, y, z, o float32)
	MoveStop(x, y, z, o float32)
	SetFacing(x, y, z, o float32)
	SetFacingMoving(x, y, z, o float32)
	SendHeartbeat(x, y, z, o float32)
	SendMovementHeartbeat(x, y, z, o float32)
	// SendMovementHeartbeatWithJump sends a moving heartbeat with MOVEMENTFLAG_FALLING set
	// and the extra jump info (used for the initial move in the observed manual log).
	SendMovementHeartbeatWithJump(x, y, z, o float32, fallTime uint32, zspeed, sinAngle, cosAngle, xyspeed float32)
	SendFallLand(x, y, z, o float32)
}

// MovementConfig tunes the client-like movement behavior.
type MovementConfig struct {
	// HeartbeatInterval is the target interval between movement heartbeats while moving.
	HeartbeatInterval time.Duration
	// TurnThresholdRad is the angle difference at which we consider a turn at waypoint.
	TurnThresholdRad float32
	// WaypointRadius is how close we must be to advance to next waypoint.
	WaypointRadius float32
	// StopAtEnd sends a final MoveStop + stationary HB when path completes.
	StopAtEnd bool
}

// DefaultMovementConfig returns sensible defaults modeled after observed client behavior (~500ms HB).
func DefaultMovementConfig() MovementConfig {
	return MovementConfig{
		HeartbeatInterval: 500 * time.Millisecond,
		TurnThresholdRad:  0.6, // ~34 degrees; smaller changes use in-place facing while moving
		WaypointRadius:    1.5,
		StopAtEnd:         true,
	}
}

// MovementController is the separate struct responsible for all player movement logic.
// It manages path following, position interpolation over simulated time, and decides
// exactly when and what movement packets to emit to match real client patterns (heartbeats,
// stops, facing changes at turns, etc).
type MovementController struct {
	sender MovementSender
	nav    navigation.Navigator // optional, used for ground height snapping if present
	cfg    MovementConfig
	speed  float32

	// Path data (2D horizontal parameterization for speed)
	path      []navigation.Point3D
	segLens   []float32 // 2D length of each segment i -> i+1
	cumLens   []float32 // cumulative 2D length up to start of segment i
	totalDist float32
	mapID     uint32 // for GetHeight snapping on elevation changes

	// Simulation state (driven by external time ticks for determinism in tests)
	startTime  time.Time
	travelDist float32 // distance along path that should be covered by now

	isMoving bool

	// Current pose (updated during advance)
	curX, curY, curZ, curO float32

	// Last times for throttling
	lastHBTime     time.Time
	lastFacingTime time.Time

	// jump/fall info (used to reproduce the special first heartbeat seen in manual logs)
	hasJumpInfo bool
	jumpZSpeed  float32
	jumpSin     float32
	jumpCos     float32
	jumpXY      float32

	// Turn handling state machine to emulate manual client turns at ladder corners etc.
	turnPhase       int // 0=normal, 1=waiting to face, 3=will restart
	turnTargetO     float32
	turnUntil       time.Time
	nextSegIdx      int // when we decide to turn at a vertex, remember the upcoming segment
	turnFacingsSent int // how many incremental facings sent for current turn (to space them)

	// For first-move special case (jump/fall like in log)
	firstMoveSent       bool
	firstHBWithJumpSent bool
}

// NewMovementController creates a movement controller.
func NewMovementController(sender MovementSender, speed float32, nav navigation.Navigator, cfg MovementConfig) *MovementController {
	if cfg.HeartbeatInterval <= 0 {
		cfg = DefaultMovementConfig()
	}
	if speed <= 0 {
		speed = 7.0
	}
	return &MovementController{
		sender: sender,
		nav:    nav,
		cfg:    cfg,
		speed:  speed,
	}
}

// SetPath installs a new path (from pathfinder) and starts movement from the first point.
// The caller is responsible for providing a path that starts near current position.
// mapID is used (when > 0 and nav is available) to snap Z to real ground height on elevation.
func (m *MovementController) SetPath(path []navigation.Point3D, startTime time.Time, initialO float32, mapID uint32) {
	if len(path) == 0 {
		return
	}
	m.path = make([]navigation.Point3D, len(path))
	copy(m.path, path)
	m.mapID = mapID

	// Precompute 2D segment lengths and cumulatives
	n := len(m.path)
	m.segLens = make([]float32, n-1)
	m.cumLens = make([]float32, n)
	m.totalDist = 0
	for i := 0; i < n-1; i++ {
		d := m.path[i].DistanceTo2D(m.path[i+1])
		m.segLens[i] = d
		m.cumLens[i] = m.totalDist
		m.totalDist += d
	}
	m.cumLens[n-1] = m.totalDist

	m.startTime = startTime
	m.travelDist = 0
	m.isMoving = true
	m.firstMoveSent = false
	m.lastHBTime = time.Time{}
	m.turnPhase = 0
	m.lastFacingTime = time.Time{}

	p0 := m.path[0]
	m.curX, m.curY, m.curZ = p0.X, p0.Y, p0.Z
	m.curO = initialO

	// Use the Z from the path point directly (the generator already applied height correction using navmesh).
	// Avoid additional GetHeight here to prevent drastic deltas.

	// Begin moving
	m.sender.MoveStartForward(m.curX, m.curY, m.curZ, m.curO)
	m.lastHBTime = startTime
	m.firstMoveSent = true
	m.firstHBWithJumpSent = false
}

// Update advances simulated position to the time 'now' and emits any due packets (HB, turns, stops).
// Call this regularly with monotonically increasing simulated time.
func (m *MovementController) Update(now time.Time) {
	if len(m.path) < 2 {
		return
	}
	if !m.isMoving && m.turnPhase == 0 {
		return
	}

	// Compute how far we should have traveled by wall/sim time.
	elapsed := now.Sub(m.startTime).Seconds()
	targetDist := m.speed * float32(elapsed)
	if targetDist < 0 {
		targetDist = 0
	}

	// Handle turn phase first (client-like pauses for reorientation at major turns)
	if m.turnPhase != 0 {
		m.handleTurnPhase(now)
		// During turn we still may want to emit stationary HB rarely; skip normal HB for now.
		return
	}

	// Advance pose along path
	m.advanceAlongPath(targetDist)

	// Z follows the path points' heights (which include corrections from the path generator / navmesh).
	// We no longer override here to avoid large deltas from GetHeight differing from path Z.

	// Emit periodic heartbeat (aim for the interval observed in manual test ~500ms)
	if m.lastHBTime.IsZero() || now.Sub(m.lastHBTime) >= m.cfg.HeartbeatInterval {
		// If very close to end, send a final stationary one (like current logic)
		if m.travelDist+0.1 >= m.totalDist {
			m.sender.SendHeartbeat(m.curX, m.curY, m.curZ, m.curO)
		} else if m.hasJumpInfo && !m.firstHBWithJumpSent {
			// Special first moving heartbeat with jump/fall data (matches the observed manual log's initial HB)
			m.sender.SendMovementHeartbeatWithJump(m.curX, m.curY, m.curZ, m.curO, 194,
				m.jumpZSpeed, m.jumpSin, m.jumpCos, m.jumpXY)
			m.firstHBWithJumpSent = true
		} else {
			m.sender.SendMovementHeartbeat(m.curX, m.curY, m.curZ, m.curO)
		}
		m.lastHBTime = now
	}

	// Detect if we are at (or passed) a waypoint that requires a direction change.
	// If yes, schedule a client-like stop + facing turns + restart.
	m.maybeScheduleTurnAtWaypoint(now, targetDist)

	// Check arrival at final destination
	if m.travelDist >= m.totalDist-0.05 {
		m.isMoving = false
		if m.cfg.StopAtEnd {
			m.sender.MoveStop(m.curX, m.curY, m.curZ, m.curO)
			m.sender.SendHeartbeat(m.curX, m.curY, m.curZ, m.curO)
		}
	}
}

// advanceAlongPath sets curX/Y/Z/O by walking the precomputed 2D distances.
func (m *MovementController) advanceAlongPath(dist float32) {
	if dist <= 0 || len(m.path) == 0 {
		p := m.path[0]
		m.curX, m.curY = p.X, p.Y
		m.curZ = p.Z // use path Z
		return
	}
	if dist >= m.totalDist {
		last := m.path[len(m.path)-1]
		m.curX, m.curY = last.X, last.Y
		m.curZ = last.Z // use path Z
		m.travelDist = m.totalDist
		// Face toward last direction if possible
		if len(m.path) >= 2 {
			prev := m.path[len(m.path)-2]
			m.curO = float32(math.Atan2(float64(last.Y-prev.Y), float64(last.X-prev.X)))
		}
		return
	}

	m.travelDist = dist

	// Find segment
	for i := 0; i < len(m.segLens); i++ {
		segStart := m.cumLens[i]
		segEnd := m.cumLens[i] + m.segLens[i]
		if dist >= segStart && dist < segEnd {
			p0 := m.path[i]
			p1 := m.path[i+1]
			segFrac := float32(0)
			if m.segLens[i] > 1e-6 {
				segFrac = (dist - segStart) / m.segLens[i]
			}
			m.curX = p0.X + (p1.X-p0.X)*segFrac
			m.curY = p0.Y + (p1.Y-p0.Y)*segFrac
			m.curZ = p0.Z + (p1.Z-p0.Z)*segFrac // use path Z (includes generator height correction)
			// Face along current segment
			m.curO = float32(math.Atan2(float64(p1.Y-p0.Y), float64(p1.X-p0.X)))
			return
		}
	}
	// Fallback to last
	last := m.path[len(m.path)-1]
	m.curX, m.curY, m.curZ = last.X, last.Y, last.Z
	// Z taken from path (no snap to avoid wrong floor selection in multi-level areas).
}

// maybeScheduleTurnAtWaypoint looks ahead at the next waypoint and if the incoming
// and outgoing directions differ significantly, we emulate a player stopping to turn.
func (m *MovementController) maybeScheduleTurnAtWaypoint(now time.Time, targetDist float32) {
	if len(m.path) < 3 {
		return
	}
	// Find current segment we are on or just finishing
	curSeg := 0
	for i := 0; i < len(m.segLens); i++ {
		if targetDist < m.cumLens[i]+m.segLens[i] {
			curSeg = i
			break
		}
	}

	// Are we close to the *end* of current segment (i.e. a vertex)?
	distToVertex := (m.cumLens[curSeg] + m.segLens[curSeg]) - targetDist
	if distToVertex > m.cfg.WaypointRadius {
		return
	}

	// Is there a next segment after this vertex?
	nextSeg := curSeg + 1
	if nextSeg >= len(m.segLens) {
		return
	}

	// Compute direction of current seg and next seg
	dx0 := m.path[curSeg+1].X - m.path[curSeg].X
	dy0 := m.path[curSeg+1].Y - m.path[curSeg].Y
	dx1 := m.path[nextSeg+1].X - m.path[nextSeg].X
	dy1 := m.path[nextSeg+1].Y - m.path[nextSeg].Y

	// If zero length skip
	if (dx0*dx0+dy0*dy0) < 1e-6 || (dx1*dx1+dy1*dy1) < 1e-6 {
		return
	}

	ang0 := math.Atan2(float64(dy0), float64(dx0))
	ang1 := math.Atan2(float64(dy1), float64(dx1))
	delta := float32(math.Abs(ang1 - ang0))
	if delta > math.Pi {
		delta = 2*math.Pi - delta
	}

	if delta < m.cfg.TurnThresholdRad {
		// Small change: allow facing correction while moving (real client does this too).
		// Throttle to avoid flooding like a real client on a dense path.
		targetO := float32(ang1)
		deltaO := float32(math.Abs(float64(m.curO - targetO)))
		sinceFacing := time.Since(m.lastFacingTime)
		if deltaO > 0.12 && (m.lastFacingTime.IsZero() || sinceFacing > 70*time.Millisecond) {
			m.sender.SetFacingMoving(m.curX, m.curY, m.curZ, targetO)
			m.curO = targetO
			m.lastFacingTime = now
		}
		return
	}

	// Large turn -> schedule stop + turn + restart, similar to observed manual client behavior
	// We set state so that next Updates will emit the STOP / multiple SET_FACING / START
	m.turnPhase = 1
	m.turnTargetO = float32(ang1)
	m.turnUntil = now.Add(60 * time.Millisecond)
	m.nextSegIdx = nextSeg
	m.turnFacingsSent = 0 // reset counter for spaced facings

	// Immediately send the stop at current (near vertex) position
	m.sender.MoveStop(m.curX, m.curY, m.curZ, m.curO)
	m.isMoving = false // pause simulation advance until turn done
}

// handleTurnPhase advances the stop+face+start sequence over simulated time.
func (m *MovementController) handleTurnPhase(now time.Time) {
	switch m.turnPhase {
	case 1: // stopping done, time to send next facing (spaced across ticks like manual input)
		if now.After(m.turnUntil) {
			o := m.curO
			delta := m.turnTargetO - o
			for delta < -math.Pi {
				delta += 2 * math.Pi
			}
			for delta > math.Pi {
				delta -= 2 * math.Pi
			}
			// Emit one incremental facing per time slice
			stepsRemaining := 3 - m.turnFacingsSent
			if stepsRemaining < 1 {
				stepsRemaining = 1
			}
			step := delta / float32(stepsRemaining)
			o += step
			m.sender.SetFacing(m.curX, m.curY, m.curZ, o)
			m.curO = o
			m.turnFacingsSent++

			if m.turnFacingsSent >= 3 {
				m.turnPhase = 3
				m.turnUntil = now.Add(40 * time.Millisecond)
			} else {
				// schedule next facing soon
				m.turnUntil = now.Add(70 * time.Millisecond)
			}
		}
	case 3: // restart forward
		if now.After(m.turnUntil) {
			// resume at the vertex; advance our logical travelDist to start of next seg
			vertexDist := m.cumLens[m.nextSegIdx]
			m.travelDist = vertexDist
			p := m.path[m.nextSegIdx]
			m.curX, m.curY, m.curZ = p.X, p.Y, p.Z
			m.curO = m.turnTargetO
			// Do not snap - use path Z to stay on correct floor/level per navmesh.

			m.sender.MoveStartForward(m.curX, m.curY, m.curZ, m.curO)
			m.lastHBTime = now
			m.lastFacingTime = now
			m.isMoving = true
			m.turnPhase = 0
			m.turnFacingsSent = 0
			m.startTime = now.Add(-time.Duration((m.travelDist / m.speed) * float32(time.Second))) // adjust base so elapsed math continues correctly
		}
	}
}

// Stop forces a stop.
func (m *MovementController) Stop(now time.Time) {
	if !m.isMoving {
		return
	}
	m.isMoving = false
	m.turnPhase = 0
	m.sender.MoveStop(m.curX, m.curY, m.curZ, m.curO)
	m.sender.SendHeartbeat(m.curX, m.curY, m.curZ, m.curO)
}

// CurrentPosition returns the controller's view of current location (for assertions in tests).
func (m *MovementController) CurrentPosition() (x, y, z, o float32) {
	return m.curX, m.curY, m.curZ, m.curO
}

// IsMoving reports if still following path.
func (m *MovementController) IsMoving() bool { return m.isMoving }

// TravelDist returns how far along the path (2D) we have simulated.
func (m *MovementController) TravelDist() float32 { return m.travelDist }

// initPositionFromWorld seeds the controller's internal position with the current world position
// right after creation / login. This prevents zeroing out the spawn location before the first path.
func (m *MovementController) initPositionFromWorld(x, y, z, o float32) {
	m.curX = x
	m.curY = y
	m.curZ = z
	m.curO = o
}

// SetInitialJumpInfo configures the jump/fall trajectory data to be sent with the
// very first movement heartbeat (reproduces the special first HB with j_* fields
// seen in manual movement logs when a small drop/fall occurs right after starting forward).
func (m *MovementController) SetInitialJumpInfo(zspeed, sinAngle, cosAngle, xyspeed float32) {
	m.hasJumpInfo = true
	m.jumpZSpeed = zspeed
	m.jumpSin = sinAngle
	m.jumpCos = cosAngle
	m.jumpXY = xyspeed
	m.firstHBWithJumpSent = false
}

// ============================================================
// WorldClient adapter so Bot can drive movement via the controller.
// ============================================================

// worldMovementSender adapts a *WorldClient to the MovementSender interface.
type worldMovementSender struct {
	w *WorldClient
}

func (s *worldMovementSender) MoveStartForward(x, y, z, o float32) {
	s.w.posX, s.w.posY, s.w.posZ, s.w.orientation = x, y, z, o
	_ = s.w.MoveForward()
}

func (s *worldMovementSender) MoveStop(x, y, z, o float32) {
	s.w.posX, s.w.posY, s.w.posZ, s.w.orientation = x, y, z, o
	_ = s.w.MoveStop()
}

func (s *worldMovementSender) SetFacing(x, y, z, o float32) {
	s.w.posX, s.w.posY, s.w.posZ, s.w.orientation = x, y, z, o
	_ = s.w.SetFacing(o)
}

func (s *worldMovementSender) SetFacingMoving(x, y, z, o float32) {
	s.w.posX, s.w.posY, s.w.posZ, s.w.orientation = x, y, z, o
	_ = s.w.SetFacingMoving(o)
}

func (s *worldMovementSender) SendHeartbeat(x, y, z, o float32) {
	s.w.posX, s.w.posY, s.w.posZ, s.w.orientation = x, y, z, o
	_ = s.w.SendHeartbeat()
}

func (s *worldMovementSender) SendMovementHeartbeat(x, y, z, o float32) {
	s.w.posX, s.w.posY, s.w.posZ, s.w.orientation = x, y, z, o
	_ = s.w.SendMovementHeartbeat()
}

func (s *worldMovementSender) SendMovementHeartbeatWithJump(x, y, z, o float32, fallTime uint32, zspeed, sinAngle, cosAngle, xyspeed float32) {
	s.w.posX, s.w.posY, s.w.posZ, s.w.orientation = x, y, z, o
	_ = s.w.SendMovementHeartbeatWithJump(fallTime, zspeed, sinAngle, cosAngle, xyspeed)
}

func (s *worldMovementSender) SendFallLand(x, y, z, o float32) {
	s.w.posX, s.w.posY, s.w.posZ, s.w.orientation = x, y, z, o
	// Fall land is a specific opcode; send via generic if available or heartbeat as approximation for now.
	// For fidelity we can expose from world if needed. Use stationary HB with position.
	_ = s.w.SendHeartbeat()
}
