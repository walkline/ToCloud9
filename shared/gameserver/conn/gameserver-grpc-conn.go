package conn

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"

	"github.com/walkline/ToCloud9/gen/worldserver/pb"
)

type GameServerGRPCConnMgr interface {
	AddAddressMapping(gameServerAddress, grpcServerAddress string)
	GRPCAddressForGameServer(gameServerAddress string) string
	GRPCConnByGameServerAddress(address string) (conn pb.WorldServerServiceClient, err error)
}

type gameServerGRPCConnMgrImpl struct {
	addressesMapping map[ /*gameServerAddress*/ string] /*grpcAddress*/ string
	addressWithConn  map[string]pb.WorldServerServiceClient
	lock             sync.RWMutex
}

var DefaultGameServerGRPCConnMgr = NewGameServerGRPCConnMgr()

func NewGameServerGRPCConnMgr() GameServerGRPCConnMgr {
	return &gameServerGRPCConnMgrImpl{
		addressesMapping: map[string]string{},
		addressWithConn:  map[string]pb.WorldServerServiceClient{},
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

func (m *gameServerGRPCConnMgrImpl) GRPCConnByGameServerAddress(address string) (conn pb.WorldServerServiceClient, err error) {
	m.lock.RLock()
	connAddress := m.addressesMapping[address]
	conn = m.addressWithConn[connAddress]
	m.lock.RUnlock()

	if connAddress == "" {
		return nil, fmt.Errorf("game server grpc address is empty for address '%v'", address)
	}

	if conn == nil {
		conn, err = m.establishConn(connAddress)
		if err == nil {
			m.lock.Lock()
			m.addressWithConn[connAddress] = conn
			m.lock.Unlock()
		}
	}

	return conn, err
}

func (m *gameServerGRPCConnMgrImpl) establishConn(address string) (pb.WorldServerServiceClient, error) {
	conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
		dialer := net.Dialer{Timeout: time.Second * 5}
		return dialer.DialContext(ctx, "tcp", s)
	}))
	if err != nil {
		return nil, fmt.Errorf("can't connect to gameserver grpc server, err: %w", err)
	}

	return pb.NewWorldServerServiceClient(conn), nil
}
