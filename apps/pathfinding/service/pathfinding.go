package service

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"
	"sync"

	"github.com/o0olele/detour-go/detour"
)

const (
	maxPathLength      = 74
	maxPointPathLength = 74
	smoothPathStepSize = float32(4.0)
	smoothPathSlop     = float32(0.3)
	vertexSize         = 3
	invalidPolyRef     = detour.DtPolyRef(0)

	GROUND_HEIGHT_TOLERANCE = 0.05
	DEFAULT_HEIGHT_SEARCH   = 50.0
	// Z_OFFSET_FIND_HEIGHT matches AzerothCore SharedDefines.h
	Z_OFFSET_FIND_HEIGHT = 2.0
	// DEFAULT_COLLISION_HEIGHT matches AzerothCore ObjectDefines.h (most common model value)
	DEFAULT_COLLISION_HEIGHT = 2.03128
)

func fuzzyGe(a, b float32) bool {
	return a > b || math.Abs(float64(a-b)) < 0.00001
}

// PathType describes the result status of a pathfinding query.
type PathType uint32

const (
	PathfindBlank          PathType = 0x00
	PathfindNormal         PathType = 0x01
	PathfindShortcut       PathType = 0x02
	PathfindIncomplete     PathType = 0x04
	PathfindNopath         PathType = 0x08
	PathfindNotUsingPath   PathType = 0x10
	PathfindShort          PathType = 0x20
	PathfindFarFromPolyStart PathType = 0x40
	PathfindFarFromPolyEnd   PathType = 0x80
)

// Point3D represents a 3D point in game coordinates.
type Point3D struct {
	X, Y, Z float32
}

// PathResult contains the output of a pathfinding query.
type PathResult struct {
	Type   PathType
	Points []Point3D
}

// PathLength computes the total length of the path.
func (r *PathResult) PathLength() float32 {
	if len(r.Points) < 2 {
		return 0
	}
	var length float32
	for i := 1; i < len(r.Points); i++ {
		dx := r.Points[i].X - r.Points[i-1].X
		dy := r.Points[i].Y - r.Points[i-1].Y
		dz := r.Points[i].Z - r.Points[i-1].Z
		length += float32(math.Sqrt(float64(dx*dx + dy*dy + dz*dz)))
	}
	return length
}

// PathFinder provides pathfinding capabilities using MMap data.
// Each PathFinder instance owns a DtNavMeshQuery which is NOT thread-safe.
// Use the pool-based PathFindingService for concurrent access.
type PathFinder struct {
	mmapMgr    *MMapManager
	terrainMgr *TerrainManager
	vmapMgr    *VMapManager
	mapID      uint32
	query      *detour.DtNavMeshQuery
	navMesh    *detour.DtNavMesh
	filter     *detour.DtQueryFilter
}

// NewPathFinder creates a PathFinder for the given map.
func NewPathFinder(mmapMgr *MMapManager, terrainMgr *TerrainManager, vmapMgr *VMapManager, mapID uint32) (*PathFinder, error) {
	navMesh, err := mmapMgr.GetNavMesh(mapID)
	if err != nil {
		return nil, err
	}

	query, err := mmapMgr.CreateQuery(mapID)
	if err != nil {
		return nil, err
	}

	filter := detour.DtAllocDtQueryFilter()
	// Include all ground, water, magma areas like AC does for non-creature queries.
	filter.SetIncludeFlags(0x01 | 0x08 | 0x02) // NAV_GROUND | NAV_WATER | NAV_MAGMA
	filter.SetExcludeFlags(0)

	return &PathFinder{
		mmapMgr:    mmapMgr,
		terrainMgr: terrainMgr,
		vmapMgr:    vmapMgr,
		mapID:      mapID,
		query:      query,
		navMesh:    navMesh,
		filter:     filter,
	}, nil
}

// HaveTile checks whether the tile containing the game position is loaded in the navmesh.
func (pf *PathFinder) HaveTile(p Point3D) bool {
	return pf.haveTile(p)
}

func (pf *PathFinder) haveTile(p Point3D) bool {
	// Convert game coords to recast coords: recast = {Y, Z, X}
	point := [3]float32{p.Y, p.Z, p.X}
	var tx, ty int32
	pf.navMesh.CalcTileLoc(point[:], &tx, &ty)
	if tx < 0 || ty < 0 {
		return false
	}
	return pf.navMesh.GetTileAt(tx, ty, 0) != nil
}

const sizeOfGrids = float32(533.3333)

// gameToGridCoords converts game world coordinates to the grid coordinates used for file naming.
func gameToGridCoords(gameX, gameY float32) (int, int) {
	gx := int(32 - gameX/sizeOfGrids)
	gy := int(32 - gameY/sizeOfGrids)
	return gx, gy
}

// ensureTilesForPoint loads the tile for the given game-coordinate point.
func (pf *PathFinder) ensureTilesForPoint(p Point3D) error {
	gx, gy := gameToGridCoords(p.X, p.Y)
	if gx < 0 || gy < 0 || gx > 63 || gy > 63 {
		return nil
	}
	return pf.mmapMgr.EnsureTileLoaded(pf.mapID, int32(gx), int32(gy))
}

// ensureTilesForPath loads all tiles that might be needed for a path from start to dest.
func (pf *PathFinder) ensureTilesForPath(start, dest Point3D) error {
	sgx, sgy := gameToGridCoords(start.X, start.Y)
	dgx, dgy := gameToGridCoords(dest.X, dest.Y)

	// Determine the range of grid tiles. Expand by margin to ensure the full possible path area (including curves) has tiles loaded.
	// This fixes cases where partial tiles lead to different (suboptimal) poly paths vs AC.
	const gridMargin = 10
	minGX, maxGX := sgx, dgx
	if minGX > maxGX {
		minGX, maxGX = maxGX, minGX
	}
	minGY, maxGY := sgy, dgy
	if minGY > maxGY {
		minGY, maxGY = maxGY, minGY
	}
	minGX -= gridMargin
	maxGX += gridMargin
	minGY -= gridMargin
	maxGY += gridMargin

	for gx := minGX; gx <= maxGX; gx++ {
		for gy := minGY; gy <= maxGY; gy++ {
			if gx < 0 || gy < 0 || gx > 63 || gy > 63 {
				continue
			}
			if err := pf.mmapMgr.EnsureTileLoaded(pf.mapID, int32(gx), int32(gy)); err != nil {
				return err
			}
		}
	}
	return nil
}

// FindPath computes a path from start to dest using smooth path generation.
func (pf *PathFinder) FindPath(start, dest Point3D) (*PathResult, error) {
	// Ensure tiles are loaded for both points and all intermediate tiles.
	if err := pf.ensureTilesForPath(start, dest); err != nil {
		return nil, err
	}

	if !pf.haveTile(start) || !pf.haveTile(dest) {
		return &PathResult{
			Type:   PathfindNormal | PathfindNotUsingPath,
			Points: []Point3D{start, dest},
		}, nil
	}

	// Convert game coordinates to recast coordinates: {Y, Z, X}
	startPoint := [3]float32{start.Y, start.Z, start.X}
	endPoint := [3]float32{dest.Y, dest.Z, dest.X}

	// Find the nearest polys.
	var startPoly, endPoly detour.DtPolyRef
	var closestStart, closestEnd [3]float32
	var distToStartPoly, distToEndPoly float32

	startPoly = pf.getPolyByLocation(startPoint[:], &distToStartPoly, closestStart[:])
	endPoly = pf.getPolyByLocation(endPoint[:], &distToEndPoly, closestEnd[:])

	pathType := PathfindNormal

	if startPoly == invalidPolyRef || endPoly == invalidPolyRef {
		return &PathResult{
			Type:   PathfindNopath,
			Points: []Point3D{start, dest},
		}, nil
	}

	startFarFromPoly := distToStartPoly > 7.0
	endFarFromPoly := distToEndPoly > 7.0

	if startFarFromPoly || endFarFromPoly {
		// Snap the end point to the nearest poly.
		var snappedEnd [3]float32
		if status := pf.query.ClosestPointOnPoly(endPoly, endPoint[:], snappedEnd[:], nil); detour.DtStatusSucceed(status) {
			copy(endPoint[:], snappedEnd[:])
		}
		pathType = PathfindIncomplete
	}

	// Build polygon path.
	polyPath := make([]detour.DtPolyRef, maxPathLength)
	var polyPathSize int

	if startPoly == endPoly {
		polyPath[0] = startPoly
		polyPathSize = 1
	} else {
		status := pf.query.FindPath(startPoly, endPoly, startPoint[:], endPoint[:], pf.filter, polyPath, &polyPathSize, maxPathLength)
		if polyPathSize == 0 || detour.DtStatusFailed(status) {
			return &PathResult{
				Type:   PathfindNopath,
				Points: []Point3D{start, dest},
			}, nil
		}
	}

	if polyPath[polyPathSize-1] == endPoly && pathType != PathfindIncomplete {
		pathType = PathfindNormal
	} else {
		pathType = PathfindIncomplete
	}

	if startFarFromPoly {
		pathType |= PathfindFarFromPolyStart
	}
	if endFarFromPoly {
		pathType |= PathfindFarFromPolyEnd
	}

	// Build smooth point path.
	smoothPath := make([]float32, maxPointPathLength*vertexSize)
	var smoothPathSize int

	status := pf.findSmoothPath(startPoint[:], endPoint[:], polyPath, uint32(polyPathSize), smoothPath, &smoothPathSize, uint32(maxPointPathLength))

	// Special case: start and end on same poly, only 1 point returned.
	if polyPathSize == 1 && smoothPathSize == 1 {
		copy(smoothPath[1*vertexSize:], endPoint[:])
		smoothPathSize = 2
	} else if smoothPathSize < 2 || detour.DtStatusFailed(status) {
		return &PathResult{
			Type:   pathType | PathfindNopath,
			Points: []Point3D{start, dest},
		}, nil
	} else if smoothPathSize >= int(maxPointPathLength) {
		return &PathResult{
			Type:   pathType | PathfindShort,
			Points: []Point3D{start, dest},
		}, nil
	}

	// Convert recast coords back to game coords.
	points := make([]Point3D, smoothPathSize)
	for i := 0; i < smoothPathSize; i++ {
		points[i] = Point3D{
			X: smoothPath[i*vertexSize+2],
			Y: smoothPath[i*vertexSize],
			Z: smoothPath[i*vertexSize+1],
		}
	}

	// Set final Z for each point by querying the navmesh poly height at the *final smoothed (x,y)*.
	// This replicates using the surface the path actually follows on the mesh.
	// (AC additionally runs NormalizePath/UpdateAllowedPositionZ which may adjust further using maps/vmaps;
	// when aux data has full fidelity the getMapHeight path above would be used instead.)
	for i := range points {
		pt := [3]float32{points[i].Y, points[i].Z + 5.0, points[i].X}
		var pr detour.DtPolyRef
		var cl [3]float32
		ext := [3]float32{5, 20, 5}
		if st := pf.query.FindNearestPoly(pt[:], ext[:], pf.filter, &pr, cl[:]); detour.DtStatusSucceed(st) && pr != invalidPolyRef {
			var ph float32
			if st2 := pf.query.GetPolyHeight(pr, cl[:], &ph); detour.DtStatusSucceed(st2) {
				points[i].Z = ph
			}
		}
	}

	return &PathResult{
		Type:   pathType,
		Points: points,
	}, nil
}

// FindRandomPointAroundCircle finds a random valid point within radius of the center.
func (pf *PathFinder) FindRandomPointAroundCircle(center Point3D, radius float32) (*PathResult, error) {
	if err := pf.ensureTilesForPoint(center); err != nil {
		return nil, err
	}

	if !pf.haveTile(center) {
		return &PathResult{
			Type:   PathfindNopath,
			Points: nil,
		}, nil
	}

	// Convert to recast coords.
	centerRecast := [3]float32{center.Y, center.Z, center.X}

	// Find start poly.
	var startPoly detour.DtPolyRef
	extents := [3]float32{3.0, 5.0, 3.0}
	var closestPt [3]float32

	if status := pf.query.FindNearestPoly(centerRecast[:], extents[:], pf.filter, &startPoly, closestPt[:]); detour.DtStatusFailed(status) || startPoly == invalidPolyRef {
		// Try with larger extents.
		extents[1] = 50.0
		if status := pf.query.FindNearestPoly(centerRecast[:], extents[:], pf.filter, &startPoly, closestPt[:]); detour.DtStatusFailed(status) || startPoly == invalidPolyRef {
			return &PathResult{Type: PathfindNopath}, nil
		}
	}

	var randomRef detour.DtPolyRef
	var randomPt [3]float32
	rng := rand.New(rand.NewSource(rand.Int63()))
	frand := func() float32 { return rng.Float32() }

	status := pf.query.FindRandomPointAroundCircle(startPoly, centerRecast[:], radius, pf.filter, frand, &randomRef, randomPt[:])
	if detour.DtStatusFailed(status) {
		return &PathResult{Type: PathfindNopath}, nil
	}

	// Convert recast coords back to game coords.
	destGame := Point3D{
		X: randomPt[2],
		Y: randomPt[0],
		Z: randomPt[1],
	}

	// Now find a path from center to the random point.
	return pf.FindPath(center, destGame)
}

// getPolyByLocation finds the nearest poly to the given recast-coordinate point.
func (pf *PathFinder) getPolyByLocation(point []float32, distance *float32, closestPoint []float32) detour.DtPolyRef {
	var polyRef detour.DtPolyRef

	// Try with small extents first.
	extents := [3]float32{3.0, 5.0, 3.0}
	if status := pf.query.FindNearestPoly(point, extents[:], pf.filter, &polyRef, closestPoint); detour.DtStatusSucceed(status) && polyRef != invalidPolyRef {
		*distance = dtVdist(closestPoint, point)
		return polyRef
	}

	// Try with bigger search box.
	extents[1] = 50.0
	if status := pf.query.FindNearestPoly(point, extents[:], pf.filter, &polyRef, closestPoint); detour.DtStatusSucceed(status) && polyRef != invalidPolyRef {
		*distance = dtVdist(closestPoint, point)
		return polyRef
	}

	*distance = math.MaxFloat32
	return invalidPolyRef
}

func (pf *PathFinder) findSmoothPath(startPos, endPos []float32, polyPath []detour.DtPolyRef, polyPathSize uint32,
	smoothPath []float32, smoothPathSize *int, maxSmoothPathSize uint32) detour.DtStatus {
	*smoothPathSize = 0
	nsmoothPath := uint32(0)

	polys := make([]detour.DtPolyRef, maxPathLength)
	copy(polys, polyPath[:polyPathSize])
	npolys := polyPathSize

	var iterPos, targetPos [3]float32

	if polyPathSize > 1 {
		if detour.DtStatusFailed(pf.query.ClosestPointOnPolyBoundary(polys[0], startPos, iterPos[:])) {
			return detour.DT_FAILURE
		}
		if detour.DtStatusFailed(pf.query.ClosestPointOnPolyBoundary(polys[npolys-1], endPos, targetPos[:])) {
			return detour.DT_FAILURE
		}
	} else {
		copy(iterPos[:], startPos)
		copy(targetPos[:], endPos)
	}

	// Store first point as-is (from boundary or start). AC stores iterPos directly here; +0.5 is added only to subsequent steering points.
	dtVcopy(smoothPath[nsmoothPath*vertexSize:], iterPos[:])
	nsmoothPath++

	for npolys > 0 && nsmoothPath < maxSmoothPathSize {
		var steerPos [3]float32
		var steerPosFlag detour.DtStraightPathFlags
		var steerPosRef detour.DtPolyRef

		if !pf.getSteerTarget(iterPos[:], targetPos[:], smoothPathSlop, polys, npolys, steerPos[:], &steerPosFlag, &steerPosRef) {
			break
		}

		endOfPath := (steerPosFlag & detour.DT_STRAIGHTPATH_END) != 0
		offMeshConnection := (steerPosFlag & detour.DT_STRAIGHTPATH_OFFMESH_CONNECTION) != 0

		// Find movement delta.
		delta := [3]float32{
			steerPos[0] - iterPos[0],
			steerPos[1] - iterPos[1],
			steerPos[2] - iterPos[2],
		}
		lenVal := float32(math.Sqrt(float64(delta[0]*delta[0] + delta[1]*delta[1] + delta[2]*delta[2])))
		if (endOfPath || offMeshConnection) && lenVal < smoothPathStepSize {
			lenVal = 1.0
		} else {
			lenVal = smoothPathStepSize / lenVal
		}

		moveTgt := [3]float32{
			iterPos[0] + delta[0]*lenVal,
			iterPos[1] + delta[1]*lenVal,
			iterPos[2] + delta[2]*lenVal,
		}

		// Move.
		var result [3]float32
		const maxVisitPoly = 16
		visited := make([]detour.DtPolyRef, maxVisitPoly)
		var nvisited int

		var bHit bool
		if detour.DtStatusFailed(pf.query.MoveAlongSurface(polys[0], iterPos[:], moveTgt[:], pf.filter, result[:], visited, &nvisited, maxVisitPoly, &bHit)) {
			return detour.DT_FAILURE
		}

		npolys = pf.fixupCorridor(polys, npolys, maxPathLength, visited, uint32(nvisited))

		// Get the real surface height.
		var h float32
		pf.query.GetPolyHeight(polys[0], result[:], &h)
		result[1] = h
		result[1] += 0.5 // temp lift matches AC FindSmoothPath for steering iterations (Normalize fixes later)
		copy(iterPos[:], result[:])

		// Record: we still record the lifted for consistency during generation; normalize step after will fix to map/vmap Z.
		record := [3]float32{iterPos[0], iterPos[1], iterPos[2]}

		// Handle end of path.
		if endOfPath && inRangeYZX(iterPos[:], steerPos[:], smoothPathSlop, 1.0) {
			copy(iterPos[:], targetPos[:])
			if nsmoothPath < maxSmoothPathSize {
				// record the target as-is (AC copies targetPos directly; NormalizePath fixes Z later)
				dtVcopy(smoothPath[nsmoothPath*vertexSize:], iterPos[:])
				nsmoothPath++
			}
			break
		} else if offMeshConnection && inRangeYZX(iterPos[:], steerPos[:], smoothPathSlop, 1.0) {
			// Handle off-mesh connection.
			prevRef := invalidPolyRef
			polyRef := polys[0]
			var npos uint32
			for npos < npolys && polyRef != steerPosRef {
				prevRef = polyRef
				polyRef = polys[npos]
				npos++
			}
			for i := npos; i < npolys; i++ {
				polys[i-npos] = polys[i]
			}
			npolys -= npos

			var connStartPos, connEndPos [3]float32
			if detour.DtStatusSucceed(pf.navMesh.GetOffMeshConnectionPolyEndPoints(prevRef, polyRef, connStartPos[:], connEndPos[:])) {
				if nsmoothPath < maxSmoothPathSize {
					dtVcopy(smoothPath[nsmoothPath*vertexSize:], connStartPos[:])
					nsmoothPath++
				}
				copy(iterPos[:], connEndPos[:])
				var polyH float32
				if detour.DtStatusFailed(pf.query.GetPolyHeight(polys[0], iterPos[:], &polyH)) {
					return detour.DT_FAILURE
				}
				iterPos[1] = polyH + 0.5 // match AC temp lift
				// record version (lifted)
				if nsmoothPath < maxSmoothPathSize {
					smoothPath[nsmoothPath*vertexSize+1] = iterPos[1]
				}
			}
		}

		if nsmoothPath < maxSmoothPathSize {
			dtVcopy(smoothPath[nsmoothPath*vertexSize:], record[:])
			nsmoothPath++
		}
	}

	*smoothPathSize = int(nsmoothPath)

	if nsmoothPath < uint32(maxPointPathLength) {
		return detour.DT_SUCCESS
	}
	return detour.DT_FAILURE
}

func (pf *PathFinder) getSteerTarget(startPos, endPos []float32, minTargetDist float32, path []detour.DtPolyRef, pathSize uint32,
	steerPos []float32, steerPosFlag *detour.DtStraightPathFlags, steerPosRef *detour.DtPolyRef) bool {

	const maxSteerPoints = 3
	steerPath := make([]float32, maxSteerPoints*vertexSize)
	steerPathFlags := make([]detour.DtStraightPathFlags, maxSteerPoints)
	steerPathPolys := make([]detour.DtPolyRef, maxSteerPoints)
	var nsteerPath int

	status := pf.query.FindStraightPath(startPos, endPos, path, int(pathSize),
		steerPath, steerPathFlags, steerPathPolys, &nsteerPath, maxSteerPoints, 0)

	if nsteerPath == 0 || detour.DtStatusFailed(status) {
		return false
	}

	// Find vertex far enough to steer to. (AC does first beyond slop)
	ns := 0
	for ns < nsteerPath {
		if (steerPathFlags[ns]&detour.DT_STRAIGHTPATH_OFFMESH_CONNECTION) != 0 ||
			!inRangeYZX(steerPath[ns*vertexSize:], startPos, minTargetDist, 1000.0) {
			break
		}
		ns++
	}
	if ns >= nsteerPath {
		return false
	}

	dtVcopy(steerPos, steerPath[ns*vertexSize:])
	steerPos[1] = startPos[1] // keep Z value
	*steerPosFlag = steerPathFlags[ns]
	*steerPosRef = steerPathPolys[ns]
	return true
}

func (pf *PathFinder) fixupCorridor(path []detour.DtPolyRef, npath, maxPath uint32, visited []detour.DtPolyRef, nvisited uint32) uint32 {
	furthestPath := int32(-1)
	furthestVisited := int32(-1)

	for i := int32(npath) - 1; i >= 0; i-- {
		found := false
		for j := int32(nvisited) - 1; j >= 0; j-- {
			if path[i] == visited[j] {
				furthestPath = i
				furthestVisited = j
				found = true
			}
		}
		if found {
			break
		}
	}

	if furthestPath == -1 || furthestVisited == -1 {
		return npath
	}

	req := uint32(int32(nvisited) - furthestVisited)
	var orig uint32
	if uint32(furthestPath+1) < npath {
		orig = uint32(furthestPath + 1)
	} else {
		orig = npath
	}
	var size uint32
	if npath > orig {
		size = npath - orig
	}
	if req+size > maxPath {
		size = maxPath - req
	}

	if size > 0 {
		copy(path[req:req+size], path[orig:orig+size])
	}

	for i := uint32(0); i < req; i++ {
		path[i] = visited[nvisited-1-i]
	}

	return req + size
}

// Utility functions matching detour naming conventions.
func dtVcopy(dst, src []float32) {
	dst[0] = src[0]
	dst[1] = src[1]
	dst[2] = src[2]
}

func dtVdist(v1, v2 []float32) float32 {
	dx := v2[0] - v1[0]
	dy := v2[1] - v1[1]
	dz := v2[2] - v1[2]
	return float32(math.Sqrt(float64(dx*dx + dy*dy + dz*dz)))
}

func inRangeYZX(v1, v2 []float32, r, h float32) bool {
	dx := v2[0] - v1[0]
	dy := v2[1] - v1[1]
	dz := v2[2] - v1[2]
	return (dx*dx+dz*dz) < r*r && math.Abs(float64(dy)) < float64(h)
}

// PathFindingService is a thread-safe service that manages a pool of PathFinder instances.
type PathFindingService struct {
	mmapMgr    *MMapManager
	terrainMgr *TerrainManager
	vmapMgr    *VMapManager

	mu    sync.Mutex
	pools map[uint32]*pathFinderPool
}

type pathFinderPool struct {
	mu      sync.Mutex
	finders []*PathFinder
}

// NewPathFindingService creates a new PathFindingService from separate mmaps and maps directories.
// For convenience when using a single ac-data root containing mmaps/, maps/, vmaps/ subdirectories,
// prefer NewPathFindingServiceFromDataDir instead.
func NewPathFindingService(mmapsDir, mapsDir string) *PathFindingService {
	var tm *TerrainManager
	if mapsDir != "" {
		tm = NewTerrainManager(mapsDir)
	}
	var vm *VMapManager
	if mapsDir != "" {
		// common layout has vmaps/ next to maps/
		vmd := strings.TrimRight(mapsDir, "/") + "/../vmaps"
		if fi, err := os.Stat(vmd); err == nil && fi.IsDir() {
			vm = NewVMapManager(vmd)
		}
	}
	return &PathFindingService{
		mmapMgr:    NewMMapManager(mmapsDir),
		terrainMgr: tm,
		vmapMgr:    vm,
		pools:      make(map[uint32]*pathFinderPool),
	}
}

// NewPathFindingServiceFromDataDir creates a PathFindingService from a single data directory
// root (e.g. ac-data/) that is expected to contain:
//   - mmaps/
//   - maps/
//   - vmaps/  (optional, for accurate height)
// Subdirectories are resolved automatically.
func NewPathFindingServiceFromDataDir(dataDir string) *PathFindingService {
	if dataDir == "" {
		return NewPathFindingService("", "")
	}
	dataDir = strings.TrimRight(dataDir, "/\\") + string(os.PathSeparator)
	mmapsDir := dataDir + "mmaps"
	mapsDir := dataDir + "maps"
	return NewPathFindingService(mmapsDir, mapsDir)
}

// NewPathFindingServiceWithMMapMgr creates a new PathFindingService using an existing MMapManager (and optional terrain/vmap).
func NewPathFindingServiceWithMMapMgr(mmapMgr *MMapManager, terrainMgr *TerrainManager, vmapMgr *VMapManager) *PathFindingService {
	return &PathFindingService{
		mmapMgr:    mmapMgr,
		terrainMgr: terrainMgr,
		vmapMgr:    vmapMgr,
		pools:      make(map[uint32]*pathFinderPool),
	}
}

func (s *PathFindingService) getPool(mapID uint32) *pathFinderPool {
	s.mu.Lock()
	defer s.mu.Unlock()

	pool, ok := s.pools[mapID]
	if !ok {
		pool = &pathFinderPool{}
		s.pools[mapID] = pool
	}
	return pool
}

func (s *PathFindingService) acquirePathFinder(mapID uint32) (*PathFinder, error) {
	pool := s.getPool(mapID)

	pool.mu.Lock()
	if len(pool.finders) > 0 {
		pf := pool.finders[len(pool.finders)-1]
		pool.finders = pool.finders[:len(pool.finders)-1]
		pool.mu.Unlock()
		return pf, nil
	}
	pool.mu.Unlock()

	return NewPathFinder(s.mmapMgr, s.terrainMgr, s.vmapMgr, mapID)
}

func (s *PathFindingService) releasePathFinder(mapID uint32, pf *PathFinder) {
	pool := s.getPool(mapID)
	pool.mu.Lock()
	pool.finders = append(pool.finders, pf)
	pool.mu.Unlock()
}

// FindPath finds a path from start to dest within the same map.
func (s *PathFindingService) FindPath(mapID uint32, start, dest Point3D) (*PathResult, error) {
	pf, err := s.acquirePathFinder(mapID)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire pathfinder for map %d: %w", mapID, err)
	}
	defer s.releasePathFinder(mapID, pf)

	return pf.FindPath(start, dest)
}

// FindRandomPath finds a random valid path within radius of center.
func (s *PathFindingService) FindRandomPath(mapID uint32, center Point3D, radius float32) (*PathResult, error) {
	pf, err := s.acquirePathFinder(mapID)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire pathfinder for map %d: %w", mapID, err)
	}
	defer s.releasePathFinder(mapID, pf)

	return pf.FindRandomPointAroundCircle(center, radius)
}

// GetHeight returns the ground height (Z) at (x,y) using best available source:
// navmesh poly height (includes vmap data baked in) if possible, otherwise terrain from .map.
// This provides values aligned with what path correction and AC use.
func (s *PathFindingService) GetHeight(mapID uint32, x, y, z float32) (float32, bool) {
	pf, err := s.acquirePathFinder(mapID)
	if err != nil {
		// fallback to terrain only
		if s.terrainMgr != nil {
			return s.terrainMgr.GetHeight(mapID, x, y, z)
		}
		return 0, false
	}
	defer s.releasePathFinder(mapID, pf)
	return pf.GetHeight(x, y, z)
}

// getMapHeight replicates Map::GetHeight + WorldObject::GetMapHeight bump exactly for path normalization.
// It does NOT prefer navmesh poly height (unlike GetHeight which optimizes to avoid vmap loads).
// This produces the Z values that AC writes in NormalizePath via UpdateAllowedPositionZ/GetMap*Level.
func (pf *PathFinder) getMapHeight(x, y, z float32) (float32, bool) {
	// Bump matches WorldObject::GetMapHeight: z += max(collisionHeight, Z_OFFSET_FIND_HEIGHT)
	// We use the AC default collision for creature-based path tests (DEFAULT_COLLISION_HEIGHT).
	searchZ := z + float32(math.Max(DEFAULT_COLLISION_HEIGHT, Z_OFFSET_FIND_HEIGHT))

	mapH := float32(-100000.0)
	if pf.terrainMgr != nil {
		if gridH, ok := pf.terrainMgr.GetGridHeight(pf.mapID, x, y); ok {
			if fuzzyGe(searchZ, gridH-GROUND_HEIGHT_TOLERANCE) {
				mapH = gridH
			}
		}
	}

	vmapH := float32(-100000.0)
	if pf.vmapMgr != nil {
		if vh, ok := pf.vmapMgr.GetHeight(pf.mapID, x, y, searchZ, DEFAULT_HEIGHT_SEARCH); ok && vh > INVALID_HEIGHT {
			vmapH = vh
		}
	}

	if vmapH > -100000.0 {
		if mapH > -100000.0 {
			if vmapH > mapH || math.Abs(float64(mapH-searchZ)) > math.Abs(float64(vmapH-searchZ)) {
				return vmapH, true
			}
			return mapH, true
		}
		return vmapH, true
	}
	if mapH > -100000.0 {
		return mapH, true
	}

	// Fallback: query navmesh poly height at (x,y) using a search near input z.
	// Ensures usable Z when the test ac-data's maps report sentinel low gridH and vmaps lack coverage for the area.
	// When full AC-like aux data (vmaps or correct maps) is present this path is not reached and normalize uses it.
	pt := [3]float32{y, z + 5.0, x}
	var pr detour.DtPolyRef
	var cl [3]float32
	ext := [3]float32{5, 20, 5}
	if st := pf.query.FindNearestPoly(pt[:], ext[:], pf.filter, &pr, cl[:]); detour.DtStatusSucceed(st) && pr != invalidPolyRef {
		var ph float32
		if st2 := pf.query.GetPolyHeight(pr, cl[:], &ph); detour.DtStatusSucceed(st2) {
			return ph, true
		}
	}
	return 0, false
}

// GetHeight returns the ground Z at (x,y) using *exact* AzerothCore logic from Map::GetHeight.
// See AC Map.cpp GetHeight for the fuzzy + selection rules.
// The incoming z is the "pre-correction" value (e.g. poly height or input Z from smooth path).
// We replicate WorldObject::GetMapHeight by adding Z_OFFSET_FIND_HEIGHT to the search origin
// for vmap queries and for the closeness comparison used to pick between map/vmap.
func (pf *PathFinder) GetHeight(x, y, z float32) (float32, bool) {
	// ensure tile for poly if needed as fallback (mmap load is relatively cheap compared to vmap models)
	_ = pf.ensureTilesForPoint(Point3D{X: x, Y: y, Z: z})

	// AC WorldObject::GetMapHeight adds max(collision, Z_OFFSET) to the hint z before Map::GetHeight.
	searchZ := z + Z_OFFSET_FIND_HEIGHT

	mapHeight := float32(-100000.0)
	if pf.terrainMgr != nil {
		if gridH, ok := pf.terrainMgr.GetGridHeight(pf.mapID, x, y); ok {
			if fuzzyGe(searchZ, gridH-GROUND_HEIGHT_TOLERANCE) {
				mapHeight = gridH
			}
		}
	}

	// Prefer navmesh poly height early. This gives the surface height the path was built on,
	// and avoids triggering expensive vmap model loads (which were causing 60GB+ in load tests
	// as bots moved and GetHeight was called frequently).
	polyHeight := float32(-100000.0)
	for _, tryZ := range []float32{searchZ, searchZ + 50, searchZ + 200} {
		pt := [3]float32{y, tryZ, x}
		var pr detour.DtPolyRef
		var cl [3]float32
		ext := [3]float32{5, 20, 5}
		if st := pf.query.FindNearestPoly(pt[:], ext[:], pf.filter, &pr, cl[:]); detour.DtStatusSucceed(st) && pr != invalidPolyRef {
			var pH float32
			if st2 := pf.query.GetPolyHeight(pr, cl[:], &pH); detour.DtStatusSucceed(st2) {
				polyHeight = pH
				break
			}
		}
	}

	vmapHeight := float32(-100000.0)
	// Only hit vmapMgr (which does LoadMapTile + potential heavy model loads for hasB objects)
	// if we don't have a good poly height yet. This prevents memory bloat from loading
	// full vmap geometry for every height query during movement.
	if polyHeight <= -100000.0 && pf.vmapMgr != nil {
		if vh, ok := pf.vmapMgr.GetHeight(pf.mapID, x, y, searchZ, DEFAULT_HEIGHT_SEARCH); ok && vh > INVALID_HEIGHT {
			vmapHeight = vh
		}
	}

	// Treat poly as a "vmap-like" surface if vmap wasn't used.
	if vmapHeight <= -100000.0 && polyHeight > -100000.0 {
		vmapHeight = polyHeight
	}

	if vmapHeight > -100000.0 {
		if mapHeight > -100000.0 {
			if vmapHeight > mapHeight || math.Abs(float64(mapHeight-searchZ)) > math.Abs(float64(vmapHeight-searchZ)) {
				return vmapHeight, true
			}
			return mapHeight, true
		}
		return vmapHeight, true
	}
	if mapHeight > -100000.0 {
		return mapHeight, true
	}

	// Last resort poly query (should have been found above, but for completeness)
	if polyHeight > -100000.0 {
		return polyHeight, true
	}
	return 0, false
}