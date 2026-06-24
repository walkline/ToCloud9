package service

import (
	"math"
	"testing"
	"fmt"
)

const mmapsDir = "../../../azerothcore/env/mmaps"

// Test cases are from AzerothCore's PathGenerator output.
// Minor precision differences in path points are expected because:
//  - AC's NormalizePath() snaps Z to map surface using VMap data we don't have.
//  - Floating-point differences in smooth path stepping may produce slightly different point counts.
// We validate: path is found, start/end are correct, path length is close, and no NOPATH.

func TestFindPath_Map1_Durotar(t *testing.T) {
	svc := NewPathFindingService(mmapsDir)

	start := Point3D{X: 1568.000000, Y: -4405.870117, Z: 8.377480}
	dest := Point3D{X: 1631.720947, Y: -4394.259277, Z: 23.771362}

	result, err := svc.FindPath(1, start, dest)
	if err != nil {
		t.Fatalf("FindPath failed: %v", err)
	}

	assertPathValid(t, result, start, dest, 20, 40)
}

func TestFindPath_Map571_Northrend(t *testing.T) {
	svc := NewPathFindingService(mmapsDir)

	start := Point3D{X: 5899.160156, Y: 753.091003, Z: 652.776978}
	dest := Point3D{X: 5901.779785, Y: 752.881470, Z: 641.065430}

	result, err := svc.FindPath(571, start, dest)
	if err != nil {
		t.Fatalf("FindPath failed: %v", err)
	}

	assertPathValid(t, result, start, dest, 10, 25)
}

func TestFindPath_Map530_Outland(t *testing.T) {
	svc := NewPathFindingService(mmapsDir)

	start := Point3D{X: -176.419998, Y: 1028.530029, Z: 54.256199}
	dest := Point3D{X: -240.624344, Y: 1003.074768, Z: 62.216320}

	result, err := svc.FindPath(530, start, dest)
	if err != nil {
		t.Fatalf("FindPath failed: %v", err)
	}

	assertPathValid(t, result, start, dest, 15, 30)
}

func TestFindPath_Map0_EasternKingdoms(t *testing.T) {
	svc := NewPathFindingService(mmapsDir)

	start := Point3D{X: -14437.500000, Y: 462.808990, Z: 3.907810}
	dest := Point3D{X: -14418.383789, Y: 439.825226, Z: 10.102631}

	result, err := svc.FindPath(0, start, dest)
	if err != nil {
		t.Fatalf("FindPath failed: %v", err)
	}

	assertPathValid(t, result, start, dest, 8, 20)
}

func TestFindRandomPath(t *testing.T) {
	svc := NewPathFindingService(mmapsDir)

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
}

func TestFindPath_SharedService_ConcurrentAccess(t *testing.T) {
	svc := NewPathFindingService(mmapsDir)

	start := Point3D{X: 1568.0, Y: -4405.87, Z: 8.377}
	dest := Point3D{X: 1631.72, Y: -4394.26, Z: 23.771}

	done := make(chan bool, 4)
	for i := 0; i < 4; i++ {
		go func() {
			result, err := svc.FindPath(1, start, dest)
			if err != nil {
				t.Errorf("concurrent FindPath failed: %v", err)
			}
			if result == nil || result.Type&PathfindNopath != 0 {
				t.Errorf("expected valid path in concurrent test")
			}
			done <- true
		}()
	}

	for i := 0; i < 4; i++ {
		<-done
	}
}

func TestFindPath_InvalidMap(t *testing.T) {
	svc := NewPathFindingService(mmapsDir)

	start := Point3D{X: 0, Y: 0, Z: 0}
	dest := Point3D{X: 100, Y: 100, Z: 0}

	_, err := svc.FindPath(9999, start, dest)
	if err == nil {
		t.Fatalf("expected error for invalid map, got nil")
	}
}

func assertPathValid(t *testing.T, result *PathResult, start, dest Point3D, minPoints, maxPoints int) {
	t.Helper()

	if result.Type&PathfindNopath != 0 {
		t.Fatalf("expected a valid path, got NOPATH (type=%d)", result.Type)
	}

	if len(result.Points) < minPoints || len(result.Points) > maxPoints {
		t.Fatalf("unexpected number of path points: %d (expected %d-%d)", len(result.Points), minPoints, maxPoints)
	}

	fmt.Printf("start: %v, dst: %v\n", start, dest)

	// First point should be close to start (XY).
	if !pointsClose2D(result.Points[0], start, 1.0) {
		t.Errorf("first path point too far from start:\n  expected: (%.2f, %.2f)\n  actual:   (%.2f, %.2f)",
			start.X, start.Y, result.Points[0].X, result.Points[0].Y)
	}

	// Last point should be close to dest (XY).
	last := result.Points[len(result.Points)-1]
	if !pointsClose2D(last, dest, 1.0) {
		t.Errorf("last path point too far from dest:\n  expected: (%.2f, %.2f)\n  actual:   (%.2f, %.2f)",
			dest.X, dest.Y, last.X, last.Y)
	}

	fmt.Printf("Point 0: %v\n", result.Points[0])
	// Path should be continuous (no teleportation).
	for i := 1; i < len(result.Points); i++ {
		fmt.Printf("Point %d: %v\n", i, result.Points[i])
		segLen := dist3D(result.Points[i-1], result.Points[i])
		if segLen > 10.0 { // segments should be <= SMOOTH_PATH_STEP_SIZE (4.0) + some slack
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