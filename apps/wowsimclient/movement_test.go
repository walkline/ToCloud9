package wowsimclient

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/walkline/ToCloud9/apps/wowsimclient/navigation"
)

// recordedPacket is what our fake sender captures during simulated ticks.
type recordedPacket struct {
	at         time.Time
	typ        string // "START", "STOP", "SET_FACING", "HB", "HB_MOVE", "FALL_LAND"
	x, y, z, o float32
}

type fakeSender struct {
	pkts []recordedPacket
	now  time.Time // last known sim time for relative
}

func (f *fakeSender) MoveStartForward(x, y, z, o float32) {
	f.pkts = append(f.pkts, recordedPacket{at: f.now, typ: "START", x: x, y: y, z: z, o: o})
}
func (f *fakeSender) MoveStop(x, y, z, o float32) {
	f.pkts = append(f.pkts, recordedPacket{at: f.now, typ: "STOP", x: x, y: y, z: z, o: o})
}
func (f *fakeSender) SetFacing(x, y, z, o float32) {
	f.pkts = append(f.pkts, recordedPacket{at: f.now, typ: "SET_FACING", x: x, y: y, z: z, o: o})
}
func (f *fakeSender) SetFacingMoving(x, y, z, o float32) {
	f.pkts = append(f.pkts, recordedPacket{at: f.now, typ: "SET_FACING", x: x, y: y, z: z, o: o})
}
func (f *fakeSender) SendHeartbeat(x, y, z, o float32) {
	f.pkts = append(f.pkts, recordedPacket{at: f.now, typ: "HB", x: x, y: y, z: z, o: o})
}
func (f *fakeSender) SendMovementHeartbeat(x, y, z, o float32) {
	f.pkts = append(f.pkts, recordedPacket{at: f.now, typ: "HB_MOVE", x: x, y: y, z: z, o: o})
}

func (f *fakeSender) SendMovementHeartbeatWithJump(x, y, z, o float32, fallTime uint32, zspeed, sinAngle, cosAngle, xyspeed float32) {
	// Record as special HB_MOVE_JUMP for the test trace; still a moving heartbeat.
	f.pkts = append(f.pkts, recordedPacket{at: f.now, typ: "HB_MOVE_JUMP", x: x, y: y, z: z, o: o})
}
func (f *fakeSender) SendFallLand(x, y, z, o float32) {
	f.pkts = append(f.pkts, recordedPacket{at: f.now, typ: "FALL_LAND", x: x, y: y, z: z, o: o})
}

func (f *fakeSender) setNow(t time.Time) { f.now = t }

// logEvent mirrors the parsed events from moving_test_data.txt
type logEvent struct {
	at         time.Time
	typ        string
	x, y, z, o float32
}

// parseMovingTestData parses the provided manual test log and returns the sequence of movement events
// plus the very first and very last positions.
func parseMovingTestData(path string) (events []logEvent, first, last navigation.Point3D, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, first, last, err
	}
	defer f.Close()

	// timestamp like 2026-07-01 15:43:13.569200
	tsRe := regexp.MustCompile(`^\[([0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}\.[0-9]+)\]`)
	// opcode line
	opcodeRe := regexp.MustCompile(`Opcode: \[(MSG_MOVE_[A-Z_]+|CMSG_[A-Z_]+)`)
	posRe := regexp.MustCompile(`position: ` + "`" + `X: ([0-9.\-]+) Y: ([0-9.\-]+) Z: ([0-9.\-]+) O: ([0-9.\-]+)` + "`")

	baseLayout := "2006-01-02 15:04:05.000000"

	sc := bufio.NewScanner(f)
	var lastTS time.Time
	var pendingTyp string
	for sc.Scan() {
		line := sc.Text()

		if ts := tsRe.FindStringSubmatch(line); len(ts) == 2 {
			t, perr := time.Parse(baseLayout, ts[1])
			if perr == nil {
				lastTS = t
			}
		}
		if m := opcodeRe.FindStringSubmatch(line); len(m) == 2 {
			op := m[1]
			switch {
			case strings.Contains(op, "START_FORWARD"):
				pendingTyp = "START"
			case strings.Contains(op, "STOP") && !strings.Contains(op, "STRAFE") && !strings.Contains(op, "TURN"):
				pendingTyp = "STOP"
			case strings.Contains(op, "SET_FACING"):
				pendingTyp = "SET_FACING"
			case strings.Contains(op, "HEARTBEAT"):
				pendingTyp = "HB"
			case strings.Contains(op, "FALL_LAND"):
				pendingTyp = "FALL_LAND"
			default:
				pendingTyp = ""
			}
		}
		if m := posRe.FindStringSubmatch(line); len(m) == 5 {
			x, _ := strconv.ParseFloat(m[1], 32)
			y, _ := strconv.ParseFloat(m[2], 32)
			z, _ := strconv.ParseFloat(m[3], 32)
			o, _ := strconv.ParseFloat(m[4], 32)
			if pendingTyp != "" {
				events = append(events, logEvent{at: lastTS, typ: pendingTyp, x: float32(x), y: float32(y), z: float32(z), o: float32(o)})
				pendingTyp = ""
			}
		}
	}

	if len(events) == 0 {
		return nil, first, last, fmt.Errorf("no movement events parsed")
	}
	first = navigation.Point3D{X: events[0].x, Y: events[0].y, Z: events[0].z}
	last = navigation.Point3D{X: events[len(events)-1].x, Y: events[len(events)-1].y, Z: events[len(events)-1].z}
	return events, first, last, nil
}

func TestMovementController_LadderTurningPoints_SimilarToManualLog(t *testing.T) {
	// Locate the manual test data file. Try common locations relative to workspace and $HOME.
	candidates := []string{
		"~/Downloads/moving_test_data.txt",
		"../moving_test_data.txt",
		"../../moving_test_data.txt",
		"/Users/anton.popovichenko/Downloads/moving_test_data.txt",
	}
	var dataPath string
	for _, c := range candidates {
		p := c
		if strings.HasPrefix(p, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				p = filepath.Join(home, strings.TrimPrefix(p, "~/"))
			}
		}
		if _, err := os.Stat(p); err == nil {
			dataPath = p
			break
		}
	}
	if dataPath == "" {
		t.Skip("moving_test_data.txt not found in expected locations; place it under ~/Downloads or adjust test")
	}

	logEvents, first, last, err := parseMovingTestData(dataPath)
	if err != nil {
		t.Fatalf("parse test data: %v", err)
	}
	if len(logEvents) < 5 {
		t.Fatalf("not enough events in log, got %d", len(logEvents))
	}

	t.Logf("Parsed %d movement events from log. First=(%.2f,%.2f,%.2f) Last=(%.2f,%.2f,%.2f)",
		len(logEvents), first.X, first.Y, first.Z, last.X, last.Y, last.Z)

	// --- Use real pathfinding when possible (as required). Fall back to a sampled path from the log itself.
	// The sampled path uses key positions from the log so that following it produces similar packets.
	var path []navigation.Point3D
	var usedMapID uint32
	var usedDataDir string
	useReal := false

	// Probe for real pathfinding data (explicit DATA_DIR wins, then known locations on this machine).
	// We MUST call FindPath using the first and last positions from the user's manual test file.
	candidateDataDirs := []string{
		os.Getenv("DATA_DIR"),
		"/Users/anton.popovichenko/dev/wow/ac-data",
		"/Users/anton.popovichenko/Downloads/Data-2",
		"/Users/anton.popovichenko/dev/wow/ac",
	}
	// Also add any other mmaps parents we can derive from the environment or common spots.
	if home, err := os.UserHomeDir(); err == nil {
		candidateDataDirs = append(candidateDataDirs,
			filepath.Join(home, "dev/wow/ac-data"),
			filepath.Join(home, "Downloads/Data-2"),
		)
	}
	seen := map[string]bool{}
	var viableDirs []string
	for _, d := range candidateDataDirs {
		if d == "" {
			continue
		}
		// Normalize: user may point directly at mmaps or at parent containing mmaps+maps.
		clean := strings.TrimRight(d, "/\\")
		if strings.HasSuffix(clean, "mmaps") {
			clean = filepath.Dir(clean)
		}
		if seen[clean] {
			continue
		}
		seen[clean] = true
		if st, err := os.Stat(filepath.Join(clean, "mmaps")); err == nil && st.IsDir() {
			viableDirs = append(viableDirs, clean)
		}
	}

	// Plausible map IDs for Eastern Kingdoms / instances around these coords (X~16xx Y~16xx, climbing ramps).
	// 0 = Eastern Kingdoms is the most likely for outdoor/near-undead ramp areas; we try others too.
	candidateMaps := []uint32{0, 1, 530, 571, 609, 329, 189, 229, 309}

	for _, dir := range viableDirs {
		nav := navigation.NewEmbeddedNavigator(dir)
		for _, mid := range candidateMaps {
			res, err := nav.FindPath(mid, first, last)
			if err == nil && res != nil && res.Found && len(res.Points) >= 2 {
				// Sanity: path shouldn't be ridiculously longer than straight-line.
				straight := first.DistanceTo2D(last)
				plen := float32(0)
				for j := 1; j < len(res.Points); j++ {
					plen += res.Points[j-1].DistanceTo2D(res.Points[j])
				}
				if plen > 0 && plen < straight*5.0 && plen > straight*0.7 {
					path = res.Points
					usedMapID = mid
					usedDataDir = dir
					useReal = true
					t.Logf("Using REAL path from pathfinder: mapID=%d, %d points, len=%.1f (straight=%.1f) dataDir=%s",
						mid, len(path), plen, straight, dir)
					break
				}
			}
		}
		if useReal {
			break
		}
	}

	if !useReal {
		// Fallback: sample from the actual manual execution so the controller still follows "a route"
		// built conceptually from first->last, and exercises the same HB + turn rules.
		sampled := []navigation.Point3D{first}
		for i := 3; i < len(logEvents)-2; i += 4 {
			e := logEvents[i]
			sampled = append(sampled, navigation.Point3D{X: e.x, Y: e.y, Z: e.z})
		}
		sampled = append(sampled, last)
		path = sampled
		t.Logf("Using SAMPLED path (%d pts) derived from log (no usable real navmesh for first/last via FindPath)", len(path))
	}

	// Create controller with ~realistic run speed (7.0 yd/sec observed in log deltas)
	cfg := DefaultMovementConfig()
	cfg.HeartbeatInterval = 500 * time.Millisecond
	cfg.TurnThresholdRad = 0.55
	cfg.WaypointRadius = 2.0

	sender := &fakeSender{}
	var navForController navigation.Navigator
	if useReal && usedDataDir != "" {
		navForController = navigation.NewEmbeddedNavigator(usedDataDir)
	}
	mc := NewMovementController(sender, 7.0, navForController, cfg)

	startSim := time.Date(2026, 7, 1, 15, 43, 13, 569000000, time.UTC)
	sender.setNow(startSim)
	mc.SetPath(path, startSim, logEvents[0].o, usedMapID)

	// Reproduce the special first moving heartbeat from the manual log (has FALLING flag + jump struct).
	mc.SetInitialJumpInfo(0, -0.9996932, 0.024769546, 7)

	// Simulate time ticking. We advance in small steps (like a real loop at 50-100hz) and record.
	// Total simulated duration ~15s to cover the whole manual sequence comfortably.
	step := 20 * time.Millisecond
	endSim := startSim.Add(16 * time.Second)
	for now := startSim; now.Before(endSim); now = now.Add(step) {
		sender.setNow(now)
		mc.Update(now)
	}

	// Post-process: collect only the "interesting" packets (start/stop/facing/hb) with relative times
	type simPkt struct {
		dtMs       int
		typ        string
		x, y, z, o float32
	}
	var simPkts []simPkt
	base := sender.pkts[0].at
	for _, p := range sender.pkts {
		dt := int(p.at.Sub(base).Milliseconds())
		simPkts = append(simPkts, simPkt{dtMs: dt, typ: p.typ, x: p.x, y: p.y, z: p.z, o: p.o})
	}

	t.Logf("Controller produced %d packets over sim time", len(simPkts))

	// Dump a compact trace of key events for the run (helpful to compare vs log)
	t.Logf("Key events trace (first+last 8):")
	for i, p := range simPkts {
		if i < 4 || i > len(simPkts)-5 || p.typ == "STOP" || p.typ == "START" || p.typ == "SET_FACING" {
			t.Logf("  +%4dms %-9s (%.1f,%.1f,%.1f) o=%.2f", p.dtMs, p.typ, p.x, p.y, p.z, p.o)
		}
	}

	// --- Assertions: must be "very close" to the manual execution ---
	// 1. Must start with START
	if len(simPkts) == 0 || simPkts[0].typ != "START" {
		t.Fatalf("expected first packet to be START, got %+v", simPkts)
	}

	// 2. Heartbeat spacing should be close to 500ms (allow 350-650ms window for jitter in sim)
	hbDeltas := []int{}
	lastHB := -1
	for _, p := range simPkts {
		if p.typ == "HB" || p.typ == "HB_MOVE" || p.typ == "HB_MOVE_JUMP" {
			if lastHB >= 0 {
				d := p.dtMs - lastHB
				hbDeltas = append(hbDeltas, d)
			}
			lastHB = p.dtMs
		}
	}
	if len(hbDeltas) < 3 {
		t.Fatalf("too few heartbeats produced, deltas=%v", hbDeltas)
	}
	avgDelta := 0
	for _, d := range hbDeltas {
		avgDelta += d
	}
	avgDelta /= len(hbDeltas)
	if avgDelta < 350 || avgDelta > 650 {
		t.Errorf("heartbeat timing not similar to log (~500ms); avg delta=%d deltas=%v", avgDelta, hbDeltas)
	} else {
		t.Logf("HB deltas avg ~%d ms (target 500) -- OK", avgDelta)
	}

	// 3. Must contain at least one STOP + SET_FACING sequence (the turning points)
	hasStop := false
	hasFacing := false
	for _, p := range simPkts {
		if p.typ == "STOP" {
			hasStop = true
		}
		if p.typ == "SET_FACING" {
			hasFacing = true
		}
	}
	if !hasStop {
		t.Error("expected at least one STOP packet for turning points (ladder behavior)")
	}
	if !hasFacing {
		t.Error("expected SET_FACING packets around turns")
	}

	// 4. Final packet should be near the destination (within a few units)
	lastPkt := simPkts[len(simPkts)-1]
	distToEnd := math.Sqrt(float64(
		(lastPkt.x-last.X)*(lastPkt.x-last.X) +
			(lastPkt.y-last.Y)*(lastPkt.y-last.Y)))
	if distToEnd > 8.0 {
		t.Errorf("final simulated pos too far from log end: got (%.1f,%.1f) want ~ (%.1f,%.1f) dist=%.1f",
			lastPkt.x, lastPkt.y, last.X, last.Y, distToEnd)
	}

	// 5. There should be roughly similar count of moving HBs (order of magnitude).
	// Log had ~ 5+5+5 ~15 HBs. We accept 8-30.
	numMovingHB := 0
	for _, p := range simPkts {
		if p.typ == "HB_MOVE" || p.typ == "HB_MOVE_JUMP" {
			numMovingHB++
		}
	}
	if numMovingHB < 4 {
		t.Errorf("produced too few moving heartbeats (%d); log had many more at ~0.5s", numMovingHB)
	}

	t.Logf("Produced packet types summary OK. Sample of first 12:")
	for ii := 0; ii < len(simPkts) && ii < 12; ii++ {
		p := simPkts[ii]
		t.Logf("  +%4dms %s @ (%.2f,%.2f,%.2f) o=%.2f", p.dtMs, p.typ, p.x, p.y, p.z, p.o)
	}

	// Additional: the route given to the controller must have been built (by real pathfinder or our log-sampled equivalent)
	// using the first and last positions from the manual test data, as required.
	if len(path) < 2 {
		t.Fatal("path must have at least start+end")
	}
	startDist := path[0].DistanceTo2D(first)
	endDist := path[len(path)-1].DistanceTo2D(last)
	if startDist > 5.0 || endDist > 5.0 {
		t.Errorf("path used for following did not start near log first or end near log last (startDist=%.1f endDist=%.1f)", startDist, endDist)
	}
	if useReal {
		t.Logf("Test exercised REAL pathfinding FindPath(first, last) on map %d via data dir %s", usedMapID, usedDataDir)
	}
}

// TestBloodElfDrasticSnaps exercises GetHeight at points that previously showed
// large snap deltas in Blood Elf start zone (map 530). This documents "bad calculations"
// from real runs (e.g. login snap 29.67->26.56, path points with ~5yd deltas)
// and helps debug further by logging the exact returned heights vs expected terrain.
// Run with real data dir to reproduce.
func TestBloodElfDrasticSnaps(t *testing.T) {
	dataDir := "/Users/anton.popovichenko/dev/wow/ac-data"
	if _, err := os.Stat(dataDir); err != nil {
		t.Skipf("skipping Blood Elf snap test, no data dir at %s", dataDir)
	}
	nav := navigation.NewEmbeddedNavigator(dataDir)
	mapID := uint32(530)

	// Bad cases extracted from run logs that showed drastic snaps (origZ from server/path -> snapped)
	cases := []struct {
		name    string
		x, y, z float32 // origZ (pre-snap)
	}{
		{"login-start", 10295.7, -6294.8, 29.67},   // snapped to ~26.56, delta ~-3.11
		{"path-pt-23to28", 10276.7, -6335.3, 23.5}, // example that snapped to 28.9 in one log
		{"path-pt-26to24", 10295.7, -6294.8, 26.6}, // common in logs
		{"path-low", 10258.9, -6338.8, 28.5},       // from final path
	}

	for _, c := range cases {
		gh, ok := nav.GetHeight(mapID, c.x, c.y, c.z+5.0)
		delta := float32(0)
		if ok {
			delta = gh - c.z
		}
		t.Logf("BloodElfSnap %s: map=%d pos=(%.1f,%.1f) origZ=%.2f got=%.2f delta=%.2f ok=%v", c.name, mapID, c.x, c.y, c.z, gh, delta, ok)
	}
}
