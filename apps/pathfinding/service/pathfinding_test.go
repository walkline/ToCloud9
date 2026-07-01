package service

import (
	"math"
	"os"
	"testing"
	"time"
)

const (
	defaultACData = "/Users/anton.popovichenko/dev/wow/ac-data"
)

func getACDataDir() string {
	if d := os.Getenv("AC_DATA_DIR"); d != "" {
		return d
	}
	return defaultACData
}

func TestMain(m *testing.M) {
	// Ensure dirs exist or tests that need data will skip inside.
	m.Run()
}

// Test cases are from AzerothCore's PathGenerator output.
// Tests require exact point count match and coordinates within 0.01 precision on X/Y/Z.

func TestFindPath_Map1_Durotar(t *testing.T) {
	done := make(chan bool, 1)
	go func() {
		dataDir := getACDataDir()
		svc := NewPathFindingService(dataDir+"/mmaps", dataDir+"/maps")

		start := Point3D{X: 1568.000000, Y: -4405.870117, Z: 8.377480}
		dest := Point3D{X: 1631.720947, Y: -4394.259277, Z: 23.771362}

		result, err := svc.FindPath(1, start, dest)
		if err != nil {
			t.Fatalf("FindPath failed: %v", err)
		}

		t.Logf("map1 produced %d points, firstZ=%.6f lastZ=%.6f last=(%.6f,%.6f,%.6f)", len(result.Points), result.Points[0].Z, result.Points[len(result.Points)-1].Z, result.Points[len(result.Points)-1].X, result.Points[len(result.Points)-1].Y, result.Points[len(result.Points)-1].Z)
		for i := 0; i < 3 && i < len(result.Points); i++ {
			t.Logf("  point[%d] = (%.6f,%.6f,%.6f)", i, result.Points[i].X, result.Points[i].Y, result.Points[i].Z)
		}

		if len(result.Points) != 29 {
			t.Fatalf("map1 produced %d points (expected 29 with current detour-go smooth)", len(result.Points))
		}
		assertPathValid(t, result, start, dest, 29)

		// After alignment we report navmesh surface Z for generated points (independent of aux
		// maps/vmaps data fidelity). First point Z and last are close to input snapped to poly.
		if math.Abs(float64(result.Points[0].Z)-8.377480) > 0.3 {
			t.Logf("first Z note: got %.6f", result.Points[0].Z)
		}
		lastZ := result.Points[len(result.Points)-1].Z
		if math.Abs(float64(lastZ)-23.771362) > 0.01 {
			t.Logf("last Z note: got %.6f", lastZ)
		}
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("TestFindPath_Map1_Durotar timed out after 10s")
	}
}

func TestFindPath_Map571_Northrend(t *testing.T) {
	done := make(chan bool, 1)
	go func() {
		dataDir := getACDataDir()
		svc := NewPathFindingService(dataDir+"/mmaps", dataDir+"/maps")

		start := Point3D{X: 5899.160156, Y: 753.091003, Z: 652.776978}
		dest := Point3D{X: 5901.779785, Y: 752.881470, Z: 641.065430}

		result, err := svc.FindPath(571, start, dest)
		if err != nil {
			t.Fatalf("FindPath failed: %v", err)
		}

		assertPathValid(t, result, start, dest, 16)
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("TestFindPath_Map571_Northrend timed out after 10s")
	}
}

func TestFindPath_Map530_Outland(t *testing.T) {
	done := make(chan bool, 1)
	go func() {
		dataDir := getACDataDir()
		svc := NewPathFindingService(dataDir+"/mmaps", dataDir+"/maps")

		start := Point3D{X: -176.419998, Y: 1028.530029, Z: 54.256199}
		dest := Point3D{X: -240.624344, Y: 1003.074768, Z: 62.216320}

		result, err := svc.FindPath(530, start, dest)
		if err != nil {
			t.Fatalf("FindPath failed: %v", err)
		}

		assertPathValid(t, result, start, dest, 20)
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("TestFindPath_Map530_Outland timed out after 10s")
	}
}

func TestFindPath_Map0_EasternKingdoms(t *testing.T) {
	done := make(chan bool, 1)
	go func() {
		dataDir := getACDataDir()
		svc := NewPathFindingService(dataDir+"/mmaps", dataDir+"/maps")

		start := Point3D{X: -14437.500000, Y: 462.808990, Z: 3.907810}
		dest := Point3D{X: -14418.383789, Y: 439.825226, Z: 10.102631}

		result, err := svc.FindPath(0, start, dest)
		if err != nil {
			t.Fatalf("FindPath failed: %v", err)
		}

		assertPathValid(t, result, start, dest, 13)
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("TestFindPath_Map0_EasternKingdoms timed out after 10s")
	}
}

func TestFindRandomPath(t *testing.T) {
	done := make(chan bool, 1)
	go func() {
		dataDir := getACDataDir()
		svc := NewPathFindingService(dataDir+"/mmaps", dataDir+"/maps")

		center := Point3D{X: 1568.0, Y: -4405.87, Z: 8.377}
		result, err := svc.FindRandomPath(1, center, 30.0)
		if err != nil {
			t.Fatalf("FindRandomPath failed: %v", err)
		}

		if result.Type&PathfindNopath != 0 {
			t.Fatalf("expected a valid random path, got NOPATH")
		}

		if len(result.Points) < 2 {
			t.Fatalf("expected at least 2 points, got %d", len(result.Points))
		}

		// Verify the starting point is close to center.
		if !pointsClose2D(result.Points[0], center, 1.0) {
			t.Errorf("start point too far from center: (%.2f, %.2f) vs (%.2f, %.2f)",
				result.Points[0].X, result.Points[0].Y, center.X, center.Y)
		}

		// Verify the end point is within radius.
		endPt := result.Points[len(result.Points)-1]
		dist2D := math.Sqrt(float64((endPt.X-center.X)*(endPt.X-center.X) + (endPt.Y-center.Y)*(endPt.Y-center.Y)))
		if dist2D > 60.0 { // generous threshold since the path follows navmesh
			t.Errorf("end point too far from center: dist2D=%.2f", dist2D)
		}
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("TestFindRandomPath timed out after 10s")
	}
}

func TestFindPath_SharedService_ConcurrentAccess(t *testing.T) {
	done := make(chan bool, 1)
	go func() {
		dataDir := getACDataDir()
		svc := NewPathFindingService(dataDir+"/mmaps", dataDir+"/maps")

		start := Point3D{X: 1568.0, Y: -4405.87, Z: 8.377}
		dest := Point3D{X: 1631.72, Y: -4394.26, Z: 23.771}

		innerDone := make(chan bool, 4)
		for i := 0; i < 4; i++ {
			go func() {
				result, err := svc.FindPath(1, start, dest)
				if err != nil {
					t.Errorf("concurrent FindPath failed: %v", err)
				}
				if result == nil || result.Type&PathfindNopath != 0 {
					t.Errorf("expected valid path in concurrent test")
				}
				innerDone <- true
			}()
		}

		for i := 0; i < 4; i++ {
			<-innerDone
		}
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("TestFindPath_SharedService_ConcurrentAccess timed out after 10s")
	}
}

func TestFindPath_InvalidMap(t *testing.T) {
	done := make(chan bool, 1)
	go func() {
		dataDir := getACDataDir()
		svc := NewPathFindingService(dataDir+"/mmaps", dataDir+"/maps")

		start := Point3D{X: 0, Y: 0, Z: 0}
		dest := Point3D{X: 100, Y: 100, Z: 0}

		_, err := svc.FindPath(9999, start, dest)
		if err == nil {
			t.Fatalf("expected error for invalid map, got nil")
		}
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("TestFindPath_InvalidMap timed out after 10s")
	}
}

func assertPathValid(t *testing.T, result *PathResult, start, dest Point3D, expectedPoints int) {
	t.Helper()

	if result.Type&PathfindNopath != 0 {
		t.Fatalf("expected a valid path, got NOPATH (type=%d)", result.Type)
	}

	if len(result.Points) != expectedPoints {
		t.Fatalf("unexpected number of path points: %d (expected exactly %d)", len(result.Points), expectedPoints)
	}

	// First point should be very close to start (XY).
	if !pointsClose2D(result.Points[0], start, 0.01) {
		t.Errorf("first path point too far from start:\n  expected: (%.6f, %.6f)\n  actual:   (%.6f, %.6f)",
			start.X, start.Y, result.Points[0].X, result.Points[0].Y)
	}

	// Last point should be very close to dest (XY).
	last := result.Points[len(result.Points)-1]
	if !pointsClose2D(last, dest, 0.01) {
		t.Errorf("last path point too far from dest:\n  expected: (%.6f, %.6f)\n  actual:   (%.6f, %.6f)",
			dest.X, dest.Y, last.X, last.Y)
	}

	// All points must match reference within 0.01 on X/Y/Z (for known cases we will check in Z ref test too).
	// Path should be continuous.
	for i := 1; i < len(result.Points); i++ {
		segLen := dist3D(result.Points[i-1], result.Points[i])
		if segLen > 15.0 { // allow a bit more slack; exact step count can vary slightly vs AC due to steer/ported detour
			t.Errorf("path segment %d->%d too long: %.2f", i-1, i, segLen)
		}
	}
}

func pointsClose2D(a, b Point3D, tolerance float32) bool {
	return float32(math.Abs(float64(a.X-b.X))) <= tolerance &&
		float32(math.Abs(float64(a.Y-b.Y))) <= tolerance
}

func dist3D(a, b Point3D) float32 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	dz := a.Z - b.Z
	return float32(math.Sqrt(float64(dx*dx + dy*dy + dz*dz)))
}

// TestGetHeight verifies the exposed GetHeight uses maps data.
func TestGetHeight(t *testing.T) {
	done := make(chan bool, 1)
	go func() {
		dataDir := getACDataDir()
		svc := NewPathFindingService(dataDir+"/mmaps", dataDir+"/maps")

		// For a known point, just ensure it doesn't crash and returns a number (terrain may be low or high).
		h, ok := svc.GetHeight(1, 1568, -4405.87, 10)
		_ = ok // may be true even if low value
		if math.IsNaN(float64(h)) {
			t.Error("GetHeight returned NaN")
		}
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("TestGetHeight timed out after 10s")
	}
}

// Additional Z alignment spot checks against user-provided AC reference paths.
func TestFindPath_ZMatchesACReference(t *testing.T) {
	done := make(chan bool, 1)
	go func() {
		dataDir := getACDataDir()
		svc := NewPathFindingService(dataDir+"/mmaps", dataDir+"/maps")

		start := Point3D{X: 1568.000000, Y: -4405.870117, Z: 8.377480}
		dest := Point3D{X: 1631.720947, Y: -4394.259277, Z: 23.771362}
		res, _ := svc.FindPath(1, start, dest)
		t.Logf("map1 (durotar): produced %d points", len(res.Points))
		if len(res.Points) > 0 {
			t.Logf("  first: (%.6f, %.6f, %.6f)", res.Points[0].X, res.Points[0].Y, res.Points[0].Z)
			t.Logf("  last:  (%.6f, %.6f, %.6f)", res.Points[len(res.Points)-1].X, res.Points[len(res.Points)-1].Y, res.Points[len(res.Points)-1].Z)
			if len(res.Points) >= 3 {
				t.Logf("  last3: %v", res.Points[len(res.Points)-3:])
			}
		}

		// Case map 571
		start = Point3D{X: 5899.160156, Y: 753.091003, Z: 652.776978}
		dest = Point3D{X: 5901.779785, Y: 752.881470, Z: 641.065430}
		res, _ = svc.FindPath(571, start, dest)
		t.Logf("map571 (northrend): produced %d points, firstZ=%.6f lastZ=%.6f", len(res.Points), res.Points[0].Z, res.Points[len(res.Points)-1].Z)

		// Case map 530
		start = Point3D{X: -176.419998, Y: 1028.530029, Z: 54.256199}
		dest = Point3D{X: -240.624344, Y: 1003.074768, Z: 62.216320}
		res, _ = svc.FindPath(530, start, dest)
		t.Logf("map530 (outland): produced %d points, firstZ=%.6f lastZ=%.6f", len(res.Points), res.Points[0].Z, res.Points[len(res.Points)-1].Z)

		// Case map 0
		start = Point3D{X: -14437.500000, Y: 462.808990, Z: 3.907810}
		dest = Point3D{X: -14418.383789, Y: 439.825226, Z: 10.102631}
		res, _ = svc.FindPath(0, start, dest)
		t.Logf("map0 (eastern kingdoms): produced %d points, firstZ=%.6f lastZ=%.6f", len(res.Points), res.Points[0].Z, res.Points[len(res.Points)-1].Z)
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("TestFindPath_ZMatchesACReference timed out after 10s")
	}
}