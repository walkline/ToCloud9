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
	gService  service.GameServer
	lbService service.LoadBalancer
}

func NewServersRegistry(gService service.GameServer, lbService service.LoadBalancer) pb.ServersRegistryServiceServer {
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
		RealmID:         request.RealmID,
		AvailableMaps:   stringToAvailableMaps(request.AvailableMaps),
	}

	err := s.gService.Register(ctx, gameServer)
	if err != nil {
		return nil, err
	}

	return &pb.RegisterGameServerResponse{
		Api: ver,
	}, nil
}

func (s *serversRegistry) AvailableGameServersForMapAndRealm(ctx context.Context, request *pb.AvailableGameServersForMapAndRealmRequest) (*pb.AvailableGameServersForMapAndRealmResponse, error) {
	servers, err := s.gService.AvailableForMapAndRealm(ctx, request.MapID, request.RealmID)
	if err != nil {
		return nil, err
	}

	resultServers := make([]*pb.Server, 0, len(servers))
	for i := range servers {
		resultServers = append(resultServers, &pb.Server{
			Address: servers[i].Address,
			RealmID: servers[i].RealmID,
		})
	}

	return &pb.AvailableGameServersForMapAndRealmResponse{
		Api:         ver,
		GameServers: resultServers,
	}, nil
}

func (s *serversRegistry) ListGameServersForRealm(ctx context.Context, request *pb.ListGameServersForRealmRequest) (*pb.ListGameServersForRealmResponse, error) {
	servers, err := s.gService.ListForRealm(ctx, request.RealmID)
	if err != nil {
		return nil, err
	}

	respServers := make([]*pb.GameServerDetailed, len(servers))
	for i := range servers {
		respServers[i] = &pb.GameServerDetailed{
			Address:       servers[i].Address,
			HealthAddress: servers[i].HealthCheckAddr,
			RealmID:       servers[i].RealmID,
			AvailableMaps: servers[i].AvailableMaps,
			AssignedMaps:  servers[i].AssignedMapsToHandle,
		}
	}

	return &pb.ListGameServersForRealmResponse{
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

func (s *serversRegistry) RegisterLoadBalancer(ctx context.Context, request *pb.RegisterLoadBalancerRequest) (*pb.RegisterLoadBalancerResponse, error) {
	p, _ := peer.FromContext(ctx)

	log.Info().Interface("request", request).Msg("New request to add load balancer")

	ip := removePortFromAddress(p.Addr.String())
	lbServer := &repo.LoadBalancerServer{
		Address:         fmt.Sprintf("%s:%d", request.PreferredHostName, request.GamePort),
		HealthCheckAddr: fmt.Sprintf("%s:%d", ip, request.HealthPort),
		RealmID:         request.RealmID,
	}

	server, err := s.lbService.Register(ctx, lbServer)
	if err != nil {
		return nil, err
	}

	return &pb.RegisterLoadBalancerResponse{
		Api: ver,
		Id:  server.ID,
	}, nil
}

func (s *serversRegistry) LoadBalancerForRealms(ctx context.Context, request *pb.LoadBalancerForRealmsRequest) (*pb.LoadBalancerForRealmsResponse, error) {
	servers := make([]*pb.Server, 0, len(request.RealmIDs))

	for _, realmID := range request.RealmIDs {
		server, err := s.lbService.BalancerForRealm(ctx, realmID)
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

	return &pb.LoadBalancerForRealmsResponse{
		Api:           ver,
		LoadBalancers: servers,
	}, nil
}

func (s *serversRegistry) ListLoadBalancersForRealm(ctx context.Context, request *pb.ListLoadBalancersForRealmRequest) (*pb.ListLoadBalancersForRealmResponse, error) {
	servers, err := s.lbService.ListBalancersForRealm(ctx, request.RealmID)
	if err != nil {
		return nil, err
	}

	result := make([]*pb.LoadBalancerServerDetailed, len(servers))
	for i := range servers {
		result[i] = &pb.LoadBalancerServerDetailed{
			Id:                servers[i].ID,
			Address:           servers[i].Address,
			HealthAddress:     servers[i].HealthCheckAddr,
			RealmID:           servers[i].RealmID,
			ActiveConnections: uint32(servers[i].ActiveConnections),
		}
	}

	return &pb.ListLoadBalancersForRealmResponse{
		Api:           ver,
		LoadBalancers: result,
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
