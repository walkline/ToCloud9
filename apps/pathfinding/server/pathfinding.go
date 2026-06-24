package server

import (
	"context"

	"github.com/walkline/ToCloud9/apps/pathfinding/service"
	"github.com/walkline/ToCloud9/gen/pathfinding/pb"
)

type pathfindingServer struct {
	pb.UnimplementedPathfindingServiceServer

	svc *service.PathFindingService
}

// NewPathfindingServer creates a new gRPC pathfinding server.
func NewPathfindingServer(svc *service.PathFindingService) pb.PathfindingServiceServer {
	return &pathfindingServer{svc: svc}
}

func (s *pathfindingServer) FindPath(_ context.Context, req *pb.FindPathRequest) (*pb.FindPathResponse, error) {
	start := service.Point3D{
		X: req.Start.X,
		Y: req.Start.Y,
		Z: req.Start.Z,
	}
	dest := service.Point3D{
		X: req.Dest.X,
		Y: req.Dest.Y,
		Z: req.Dest.Z,
	}

	result, err := s.svc.FindPath(req.MapId, start, dest)
	if err != nil {
		return nil, err
	}

	return toProtoResponse(result), nil
}

func (s *pathfindingServer) FindRandomPath(_ context.Context, req *pb.FindRandomPathRequest) (*pb.FindPathResponse, error) {
	center := service.Point3D{
		X: req.Center.X,
		Y: req.Center.Y,
		Z: req.Center.Z,
	}

	result, err := s.svc.FindRandomPath(req.MapId, center, req.Radius)
	if err != nil {
		return nil, err
	}

	return toProtoResponse(result), nil
}

func toProtoResponse(result *service.PathResult) *pb.FindPathResponse {
	resp := &pb.FindPathResponse{
		PathLength: result.PathLength(),
	}

	switch {
	case result.Type&service.PathfindNopath != 0:
		resp.ResultType = pb.PathResultType_PATH_RESULT_NOPATH
	case result.Type&service.PathfindIncomplete != 0:
		resp.ResultType = pb.PathResultType_PATH_RESULT_INCOMPLETE
	default:
		resp.ResultType = pb.PathResultType_PATH_RESULT_NORMAL
	}

	resp.Points = make([]*pb.Vector3, len(result.Points))
	for i, p := range result.Points {
		resp.Points[i] = &pb.Vector3{
			X: p.X,
			Y: p.Y,
			Z: p.Z,
		}
	}

	return resp
}