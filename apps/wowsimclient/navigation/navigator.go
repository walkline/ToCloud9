// Package navigation provides pathfinding for the bot, supporting both an
// embedded mmap-based pathfinder and a remote gRPC pathfinding service.
package navigation

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pathsvc "github.com/walkline/ToCloud9/apps/pathfinding/service"
	pathpb "github.com/walkline/ToCloud9/gen/pathfinding/pb"
)

// Point3D is a 3D position in game world coordinates.
type Point3D struct {
	X, Y, Z float32
}

// DistanceTo computes 3D distance.
func (p Point3D) DistanceTo(o Point3D) float32 {
	dx := p.X - o.X
	dy := p.Y - o.Y
	dz := p.Z - o.Z
	return float32(math.Sqrt(float64(dx*dx + dy*dy + dz*dz)))
}

// DistanceTo2D computes 2D distance (ignoring Z).
func (p Point3D) DistanceTo2D(o Point3D) float32 {
	dx := p.X - o.X
	dy := p.Y - o.Y
	return float32(math.Sqrt(float64(dx*dx + dy*dy)))
}

// PathResult holds the result of a pathfinding query.
type PathResult struct {
	Found  bool
	Points []Point3D
}

// Navigator is the interface for pathfinding.
type Navigator interface {
	// FindPath computes a path from start to dest on the given map.
	FindPath(mapID uint32, start, dest Point3D) (*PathResult, error)

	// FindRandomPath finds a random walkable point near center.
	FindRandomPath(mapID uint32, center Point3D, radius float32) (*PathResult, error)

	// GetHeight returns the ground height (Z) at the given point using the pathfinding data (maps + baked vmaps).
	GetHeight(mapID uint32, x, y, z float32) (float32, bool)

	// Close releases resources.
	Close()
}

// ============================================================
// Embedded navigator (uses mmap files directly)
// ============================================================

var (
	embeddedMu   sync.Mutex
	embeddedSvcs = make(map[string]*pathsvc.PathFindingService)
)

func embeddedCacheKey(dataDir string) string {
	return dataDir
}

func getOrCreateEmbeddedService(dataDir string) *pathsvc.PathFindingService {
	embeddedMu.Lock()
	defer embeddedMu.Unlock()
	key := embeddedCacheKey(dataDir)
	if svc, ok := embeddedSvcs[key]; ok {
		return svc
	}

	// Build service without VMapManager.
	// VMap (WorldModel/GroupModel) loads full triangle+vertex data from .vmo files on demand.
	// As bots wander during long load tests this causes unbounded growth (16GB+ in inuse_space
	// from (*GroupModel).readFrom). We rely on navmesh poly heights (+ terrain grids) instead.
	var svc *pathsvc.PathFindingService
	if dataDir == "" {
		svc = pathsvc.NewPathFindingServiceWithMMapMgr(pathsvc.NewMMapManager(""), nil, nil)
	} else {
		dd := strings.TrimRight(dataDir, "/\\") + string(os.PathSeparator)
		mmapsDir := dd + "mmaps"
		mapsDir := dd + "maps"

		var tm *pathsvc.TerrainManager
		if st, err := os.Stat(mapsDir); err == nil && st.IsDir() {
			tm = pathsvc.NewTerrainManager(mapsDir)
		}
		mm := pathsvc.NewMMapManager(mmapsDir)
		svc = pathsvc.NewPathFindingServiceWithMMapMgr(mm, tm, nil /* deliberately no vmapMgr */)
	}

	embeddedSvcs[key] = svc
	return svc
}

type embeddedNavigator struct {
	svc *pathsvc.PathFindingService
}

// NewEmbeddedNavigator creates a navigator using a single data directory root
// (containing mmaps/, maps/ subdirectories).
// VMaps are intentionally not loaded to avoid massive memory usage from GroupModel
// geometry during long-running load tests.
// All navigators for the same dataDir share a single underlying service.
func NewEmbeddedNavigator(dataDir string) Navigator {
	return &embeddedNavigator{
		svc: getOrCreateEmbeddedService(dataDir),
	}
}

func (n *embeddedNavigator) FindPath(mapID uint32, start, dest Point3D) (*PathResult, error) {
	result, err := n.svc.FindPath(mapID, pathsvc.Point3D{X: start.X, Y: start.Y, Z: start.Z},
		pathsvc.Point3D{X: dest.X, Y: dest.Y, Z: dest.Z})
	if err != nil {
		return nil, err
	}
	return convertResult(result), nil
}

func (n *embeddedNavigator) FindRandomPath(mapID uint32, center Point3D, radius float32) (*PathResult, error) {
	result, err := n.svc.FindRandomPath(mapID, pathsvc.Point3D{X: center.X, Y: center.Y, Z: center.Z}, radius)
	if err != nil {
		return nil, err
	}
	return convertResult(result), nil
}

func (n *embeddedNavigator) GetHeight(mapID uint32, x, y, z float32) (float32, bool) {
	return n.svc.GetHeight(mapID, x, y, z)
}

func (n *embeddedNavigator) Close() {}

func convertResult(r *pathsvc.PathResult) *PathResult {
	if r == nil {
		return &PathResult{Found: false}
	}
	found := r.Type&pathsvc.PathfindNopath == 0 && len(r.Points) > 0
	points := make([]Point3D, len(r.Points))
	for i, p := range r.Points {
		points[i] = Point3D{X: p.X, Y: p.Y, Z: p.Z}
	}
	return &PathResult{Found: found, Points: points}
}

// ============================================================
// Remote navigator (gRPC client)
// ============================================================

type remoteNavigator struct {
	conn   *grpc.ClientConn
	client pathpb.PathfindingServiceClient
}

// NewRemoteNavigator creates a navigator that connects to a remote pathfinding service.
func NewRemoteNavigator(address string) (Navigator, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to pathfinding service at %s: %w", address, err)
	}

	return &remoteNavigator{
		conn:   conn,
		client: pathpb.NewPathfindingServiceClient(conn),
	}, nil
}

func (n *remoteNavigator) FindPath(mapID uint32, start, dest Point3D) (*PathResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := n.client.FindPath(ctx, &pathpb.FindPathRequest{
		MapId: mapID,
		Start: &pathpb.Vector3{X: start.X, Y: start.Y, Z: start.Z},
		Dest:  &pathpb.Vector3{X: dest.X, Y: dest.Y, Z: dest.Z},
	})
	if err != nil {
		return nil, err
	}
	return convertProtoResult(resp), nil
}

func (n *remoteNavigator) FindRandomPath(mapID uint32, center Point3D, radius float32) (*PathResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := n.client.FindRandomPath(ctx, &pathpb.FindRandomPathRequest{
		MapId:  mapID,
		Center: &pathpb.Vector3{X: center.X, Y: center.Y, Z: center.Z},
		Radius: radius,
	})
	if err != nil {
		return nil, err
	}
	return convertProtoResult(resp), nil
}

func (n *remoteNavigator) GetHeight(mapID uint32, x, y, z float32) (float32, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := n.client.GetHeight(ctx, &pathpb.GetHeightRequest{
		MapId: mapID,
		Point: &pathpb.Vector3{X: x, Y: y, Z: z},
	})
	if err != nil || resp == nil {
		return 0, false
	}
	return resp.Height, resp.Success
}

func (n *remoteNavigator) Close() {
	if n.conn != nil {
		n.conn.Close()
	}
}

func convertProtoResult(resp *pathpb.FindPathResponse) *PathResult {
	if resp == nil {
		return &PathResult{Found: false}
	}
	found := resp.ResultType != pathpb.PathResultType_PATH_RESULT_NOPATH && len(resp.Points) > 0
	points := make([]Point3D, len(resp.Points))
	for i, p := range resp.Points {
		points[i] = Point3D{X: p.X, Y: p.Y, Z: p.Z}
	}
	return &PathResult{Found: found, Points: points}
}
