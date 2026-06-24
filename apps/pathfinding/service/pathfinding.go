package service

import (
	"fmt"
	"math"
	"math/rand"
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
)

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
	mmapMgr *MMapManager
	mapID   uint32
	query   *detour.DtNavMeshQuery
	navMesh *detour.DtNavMesh
	filter  *detour.DtQueryFilter
}

// NewPathFinder creates a PathFinder for the given map.
func NewPathFinder(mmapMgr *MMapManager, mapID uint32) (*PathFinder, error) {
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
		mmapMgr: mmapMgr,
		mapID:   mapID,
		query:   query,
		navMesh: navMesh,
		filter:  filter,
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

	// Determine the range of grid tiles.
	minGX, maxGX := sgx, dgx
	if minGX > maxGX {
		minGX, maxGX = maxGX, minGX
	}
	minGY, maxGY := sgy, dgy
	if minGY > maxGY {
		minGY, maxGY = maxGY, minGY
	}

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

		var h float32
		pf.query.GetPolyHeight(polys[0], result[:], &h)
		result[1] = h + 0.5

		copy(iterPos[:], result[:])

		// Handle end of path.
		if endOfPath && inRangeYZX(iterPos[:], steerPos[:], smoothPathSlop, 1.0) {
			copy(iterPos[:], targetPos[:])
			if nsmoothPath < maxSmoothPathSize {
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
				iterPos[1] = polyH + 0.5
			}
		}

		if nsmoothPath < maxSmoothPathSize {
			dtVcopy(smoothPath[nsmoothPath*vertexSize:], iterPos[:])
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

	// Find vertex far enough to steer to.
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
	mmapMgr *MMapManager

	mu    sync.Mutex
	pools map[uint32]*pathFinderPool
}

type pathFinderPool struct {
	mu      sync.Mutex
	finders []*PathFinder
}

// NewPathFindingService creates a new PathFindingService.
func NewPathFindingService(mmapsDir string) *PathFindingService {
	return &PathFindingService{
		mmapMgr: NewMMapManager(mmapsDir),
		pools:   make(map[uint32]*pathFinderPool),
	}
}

// NewPathFindingServiceWithMMapMgr creates a new PathFindingService using an existing MMapManager.
func NewPathFindingServiceWithMMapMgr(mmapMgr *MMapManager) *PathFindingService {
	return &PathFindingService{
		mmapMgr: mmapMgr,
		pools:   make(map[uint32]*pathFinderPool),
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

	return NewPathFinder(s.mmapMgr, mapID)
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