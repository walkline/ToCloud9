package server

import (
	"context"

	"github.com/walkline/ToCloud9/apps/guidserver"
	"github.com/walkline/ToCloud9/apps/guidserver/service"
	"github.com/walkline/ToCloud9/gen/guid/pb"
)

// GuidServer is guild server that handles grpc requests.
type GuidServer struct {
	pb.UnimplementedGuidServiceServer
	guildsService service.GuidService
}

// NewGuidServer creates new GUID server that handles grpc protocol.
func NewGuidServer(s service.GuidService) pb.GuidServiceServer {
	return &GuidServer{guildsService: s}
}

// GetGUIDPool returns available GUIDs for given realm and guid type.
func (g *GuidServer) GetGUIDPool(ctx context.Context, request *pb.GetGUIDPoolRequest) (*pb.GetGUIDPoolRequestResponse, error) {
	guids, err := g.guildsService.GetGuids(ctx, request.RealmID, uint8(request.GuidType), request.DesiredPoolSize)
	if err != nil {
		return nil, err
	}

	guidsResp := make([]*pb.GetGUIDPoolRequestResponse_GuidDiapason, len(guids))
	for i := range guids {
		guidsResp[i] = &pb.GetGUIDPoolRequestResponse_GuidDiapason{
			Start: guids[i].Start,
			End:   guids[i].End,
		}
	}

	return &pb.GetGUIDPoolRequestResponse{
		Api:          guidserver.Ver,
		ReceiverGUID: guidsResp,
	}, nil
}
