package server

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/peer"

	"github.com/walkline/ToCloud9/apps/servers-registry/repo"
	"github.com/walkline/ToCloud9/apps/servers-registry/service"
	"github.com/walkline/ToCloud9/gen/servers-registry/pb"
)

const ver = "0.0.1"

type serversRegistry struct {
	pb.UnimplementedServersRegistryServiceServer
	gService  service.GameServer
	lbService service.Gateway
}

func NewServersRegistry(gService service.GameServer, lbService service.Gateway) pb.ServersRegistryServiceServer {
	return &serversRegistry{
		gService:  gService,
		lbService: lbService,
	}
}

func (s *serversRegistry) RegisterGameServer(ctx context.Context, request *pb.RegisterGameServerRequest) (*pb.RegisterGameServerResponse, error) {
	p, _ := peer.FromContext(ctx)

	log.Info().Interface("request", request).Msg("New request to add game server")

	host := removePortFromAddress(p.Addr.String())
	if request.PreferredHostName != "" {
		host = request.PreferredHostName
	}

	gameServer := &repo.GameServer{
		Address:         fmt.Sprintf("%s:%d", host, request.GamePort),
		HealthCheckAddr: fmt.Sprintf("%s:%d", host, request.HealthPort),
		GRPCAddress:     fmt.Sprintf("%s:%d", host, request.GrpcPort),
		RealmID:         request.RealmID,
		IsCrossRealm:    request.IsCrossRealm,
		AvailableMaps:   stringToAvailableMaps(request.AvailableMaps),
	}

	err := s.gService.Register(ctx, gameServer)
	if err != nil {
		return nil, err
	}

	return &pb.RegisterGameServerResponse{
		Api:          ver,
		Id:           gameServer.ID,
		AssignedMaps: gameServer.AssignedMapsToHandle,
	}, nil
}

func (s *serversRegistry) AvailableGameServersForMapAndRealm(ctx context.Context, request *pb.AvailableGameServersForMapAndRealmRequest) (*pb.AvailableGameServersForMapAndRealmResponse, error) {
	servers, err := s.gService.AvailableForMapAndRealm(ctx, request.MapID, request.RealmID, request.IsCrossRealm)
	if err != nil {
		return nil, err
	}

	resultServers := make([]*pb.Server, 0, len(servers))
	for i := range servers {
		resultServers = append(resultServers, &pb.Server{
			Address:      servers[i].Address,
			RealmID:      servers[i].RealmID,
			IsCrossRealm: servers[i].IsCrossRealm,
			GrpcAddress:  servers[i].GRPCAddress,
		})
	}

	return &pb.AvailableGameServersForMapAndRealmResponse{
		Api:         ver,
		GameServers: resultServers,
	}, nil
}

func (s *serversRegistry) ListGameServersForRealm(ctx context.Context, request *pb.ListGameServersForRealmRequest) (*pb.ListGameServersResponse, error) {
	var (
		servers []repo.GameServer
		err     error
	)

	if request.IsCrossRealm {
		servers, err = s.gService.ListOfCrossRealms(ctx)
	} else {
		servers, err = s.gService.ListForRealm(ctx, request.RealmID)
	}
	if err != nil {
		return nil, err
	}

	respServers := make([]*pb.GameServerDetailed, len(servers))
	for i := range servers {
		respServers[i] = &pb.GameServerDetailed{
			ID:                servers[i].ID,
			Address:           servers[i].Address,
			HealthAddress:     servers[i].HealthCheckAddr,
			GrpcAddress:       servers[i].GRPCAddress,
			RealmID:           servers[i].RealmID,
			IsCrossRealm:      servers[i].IsCrossRealm,
			ActiveConnections: servers[i].ActiveConnections,
			AvailableMaps:     servers[i].AvailableMaps,
			AssignedMaps:      servers[i].AssignedMapsToHandle,
			Diff: &pb.GameServerDetailed_Diff{
				Mean:         servers[i].Diff.Mean,
				Median:       servers[i].Diff.Median,
				Percentile95: servers[i].Diff.Percentile95,
				Percentile99: servers[i].Diff.Percentile99,
				Max:          servers[i].Diff.Max,
			},
		}
	}

	return &pb.ListGameServersResponse{
		Api:         ver,
		GameServers: respServers,
	}, nil
}
func (s *serversRegistry) ListAllGameServers(ctx context.Context, request *pb.ListAllGameServersRequest) (*pb.ListGameServersResponse, error) {
	servers, err := s.gService.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	respServers := make([]*pb.GameServerDetailed, len(servers))
	for i := range servers {
		respServers[i] = &pb.GameServerDetailed{
			ID:                servers[i].ID,
			Address:           servers[i].Address,
			HealthAddress:     servers[i].HealthCheckAddr,
			GrpcAddress:       servers[i].GRPCAddress,
			RealmID:           servers[i].RealmID,
			IsCrossRealm:      servers[i].IsCrossRealm,
			ActiveConnections: servers[i].ActiveConnections,
			AvailableMaps:     servers[i].AvailableMaps,
			AssignedMaps:      servers[i].AssignedMapsToHandle,
			Diff: &pb.GameServerDetailed_Diff{
				Mean:         servers[i].Diff.Mean,
				Median:       servers[i].Diff.Median,
				Percentile95: servers[i].Diff.Percentile95,
				Percentile99: servers[i].Diff.Percentile99,
				Max:          servers[i].Diff.Max,
			},
		}
	}

	return &pb.ListGameServersResponse{
		Api:         ver,
		GameServers: respServers,
	}, nil
}

func (s *serversRegistry) RandomGameServerForRealm(ctx context.Context, request *pb.RandomGameServerForRealmRequest) (*pb.RandomGameServerForRealmResponse, error) {
	server, err := s.gService.RandomServerForRealm(ctx, request.RealmID)
	if err != nil {
		return nil, err
	}

	if server == nil {
		return &pb.RandomGameServerForRealmResponse{
			Api:        ver,
			GameServer: nil,
		}, nil
	}

	return &pb.RandomGameServerForRealmResponse{
		Api: ver,
		GameServer: &pb.Server{
			Address: server.Address,
			RealmID: server.RealmID,
		},
	}, nil
}

func (s *serversRegistry) GameServerMapsLoaded(ctx context.Context, request *pb.GameServerMapsLoadedRequest) (*pb.GameServerMapsLoadedResponse, error) {
	_, err := s.gService.MapsLoadedForServer(ctx, request.GameServerID, request.MapsLoaded)
	if err != nil {
		return nil, err
	}

	return &pb.GameServerMapsLoadedResponse{
		Api: ver,
	}, nil
}

func (s *serversRegistry) RegisterGateway(ctx context.Context, request *pb.RegisterGatewayRequest) (*pb.RegisterGatewayResponse, error) {
	p, _ := peer.FromContext(ctx)

	log.Info().Interface("request", request).Msg("New request to add gateway")

	ip := removePortFromAddress(p.Addr.String())
	lbServer := &repo.GatewayServer{
		Address:         fmt.Sprintf("%s:%d", request.PreferredHostName, request.GamePort),
		HealthCheckAddr: fmt.Sprintf("%s:%d", ip, request.HealthPort),
		RealmID:         request.RealmID,
	}

	server, err := s.lbService.Register(ctx, lbServer)
	if err != nil {
		return nil, err
	}

	return &pb.RegisterGatewayResponse{
		Api: ver,
		Id:  server.ID,
	}, nil
}

func (s *serversRegistry) GatewaysForRealms(ctx context.Context, request *pb.GatewaysForRealmsRequest) (*pb.GatewaysForRealmsResponse, error) {
	servers := make([]*pb.Server, 0, len(request.RealmIDs))

	for _, realmID := range request.RealmIDs {
		server, err := s.lbService.GatewayForRealm(ctx, realmID)
		if err != nil {
			return nil, err
		}
		if server == nil {
			continue
		}

		servers = append(servers, &pb.Server{
			Address: server.Address,
			RealmID: server.RealmID,
		})
	}

	return &pb.GatewaysForRealmsResponse{
		Api:      ver,
		Gateways: servers,
	}, nil
}

func (s *serversRegistry) ListGatewaysForRealm(ctx context.Context, request *pb.ListGatewaysForRealmRequest) (*pb.ListGatewaysForRealmResponse, error) {
	servers, err := s.lbService.GatewaysForRealm(ctx, request.RealmID)
	if err != nil {
		return nil, err
	}

	result := make([]*pb.GatewayServerDetailed, len(servers))
	for i := range servers {
		result[i] = &pb.GatewayServerDetailed{
			Id:                servers[i].ID,
			Address:           servers[i].Address,
			HealthAddress:     servers[i].HealthCheckAddr,
			RealmID:           servers[i].RealmID,
			ActiveConnections: uint32(servers[i].ActiveConnections),
		}
	}

	return &pb.ListGatewaysForRealmResponse{
		Api:      ver,
		Gateways: result,
	}, nil
}

func removePortFromAddress(address string) string {
	for i := len(address) - 1; i >= 0; i-- {
		if address[i] == ':' {
			return address[:i]
		}
	}

	return address
}

func stringToAvailableMaps(s string) []uint32 {
	v := strings.Split(s, ",")
	if len(v) == 0 {
		return []uint32{}
	}

	result := make([]uint32, 0, len(v))
	for i := range v {
		r, err := strconv.Atoi(v[i])
		if err != nil {
			continue
		}

		result = append(result, uint32(r))
	}

	return result
}
