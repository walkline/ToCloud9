package conn

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/walkline/ToCloud9/gen/worldserver/pb"
)

type GameServerGRPCConnMgr interface {
	AddAddressMapping(gameServerAddress, grpcServerAddress string)
	GRPCAddressForGameServer(gameServerAddress string) string
	GRPCConnByGameServerAddress(address string) (conn pb.WorldServerServiceClient, err error)
	// InvalidateGRPCConn drops a cached client for the gameserver address so the next
	// GRPCConnByGameServerAddress redials (use after Unavailable / TransientFailure).
	InvalidateGRPCConn(gameServerAddress string)
}

type cachedConn struct {
	client pb.WorldServerServiceClient
	raw    *grpc.ClientConn
}

type gameServerGRPCConnMgrImpl struct {
	addressesMapping map[ /*gameServerAddress*/ string] /*grpcAddress*/ string
	addressWithConn  map[string]*cachedConn
	lock             sync.RWMutex
}

var DefaultGameServerGRPCConnMgr = NewGameServerGRPCConnMgr()

func NewGameServerGRPCConnMgr() GameServerGRPCConnMgr {
	return &gameServerGRPCConnMgrImpl{
		addressesMapping: map[string]string{},
		addressWithConn:  map[string]*cachedConn{},
	}
}

func (m *gameServerGRPCConnMgrImpl) AddAddressMapping(gameServerAddress, grpcServerAddress string) {
	m.lock.Lock()
	m.addressesMapping[gameServerAddress] = grpcServerAddress
	m.lock.Unlock()
}

func (m *gameServerGRPCConnMgrImpl) GRPCAddressForGameServer(gameServerAddress string) string {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.addressesMapping[gameServerAddress]
}

func (m *gameServerGRPCConnMgrImpl) InvalidateGRPCConn(gameServerAddress string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	grpcAddr := m.addressesMapping[gameServerAddress]
	if grpcAddr == "" {
		return
	}
	if c := m.addressWithConn[grpcAddr]; c != nil && c.raw != nil {
		_ = c.raw.Close()
	}
	delete(m.addressWithConn, grpcAddr)
}

func (m *gameServerGRPCConnMgrImpl) GRPCConnByGameServerAddress(address string) (conn pb.WorldServerServiceClient, err error) {
	m.lock.RLock()
	connAddress := m.addressesMapping[address]
	cached := m.addressWithConn[connAddress]
	m.lock.RUnlock()

	if connAddress == "" {
		return nil, fmt.Errorf("game server grpc address is empty for address '%v'", address)
	}

	if cached != nil && cached.raw != nil {
		st := cached.raw.GetState()
		if st != connectivity.TransientFailure && st != connectivity.Shutdown {
			return cached.client, nil
		}
		// Drop dead connection and redial below.
		m.InvalidateGRPCConn(address)
	}

	cached, err = m.establishConn(connAddress)
	if err != nil {
		return nil, err
	}
	m.lock.Lock()
	m.addressWithConn[connAddress] = cached
	m.lock.Unlock()
	return cached.client, nil
}

func (m *gameServerGRPCConnMgrImpl) establishConn(address string) (*cachedConn, error) {
	raw, err := grpc.Dial(address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			dialer := net.Dialer{Timeout: time.Second * 5}
			return dialer.DialContext(ctx, "tcp", s)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("can't connect to gameserver grpc server, err: %w", err)
	}

	return &cachedConn{
		client: pb.NewWorldServerServiceClient(raw),
		raw:    raw,
	}, nil
}
