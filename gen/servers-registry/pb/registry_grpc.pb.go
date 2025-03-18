// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v3.20.3
// source: registry.proto

package pb

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	ServersRegistryService_RegisterGameServer_FullMethodName                 = "/v1.ServersRegistryService/RegisterGameServer"
	ServersRegistryService_AvailableGameServersForMapAndRealm_FullMethodName = "/v1.ServersRegistryService/AvailableGameServersForMapAndRealm"
	ServersRegistryService_RandomGameServerForRealm_FullMethodName           = "/v1.ServersRegistryService/RandomGameServerForRealm"
	ServersRegistryService_ListGameServersForRealm_FullMethodName            = "/v1.ServersRegistryService/ListGameServersForRealm"
	ServersRegistryService_ListAllGameServers_FullMethodName                 = "/v1.ServersRegistryService/ListAllGameServers"
	ServersRegistryService_GameServerMapsLoaded_FullMethodName               = "/v1.ServersRegistryService/GameServerMapsLoaded"
	ServersRegistryService_RegisterGateway_FullMethodName                    = "/v1.ServersRegistryService/RegisterGateway"
	ServersRegistryService_GatewaysForRealms_FullMethodName                  = "/v1.ServersRegistryService/GatewaysForRealms"
	ServersRegistryService_ListGatewaysForRealm_FullMethodName               = "/v1.ServersRegistryService/ListGatewaysForRealm"
)

// ServersRegistryServiceClient is the client API for ServersRegistryService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ServersRegistryServiceClient interface {
	RegisterGameServer(ctx context.Context, in *RegisterGameServerRequest, opts ...grpc.CallOption) (*RegisterGameServerResponse, error)
	AvailableGameServersForMapAndRealm(ctx context.Context, in *AvailableGameServersForMapAndRealmRequest, opts ...grpc.CallOption) (*AvailableGameServersForMapAndRealmResponse, error)
	RandomGameServerForRealm(ctx context.Context, in *RandomGameServerForRealmRequest, opts ...grpc.CallOption) (*RandomGameServerForRealmResponse, error)
	ListGameServersForRealm(ctx context.Context, in *ListGameServersForRealmRequest, opts ...grpc.CallOption) (*ListGameServersResponse, error)
	ListAllGameServers(ctx context.Context, in *ListAllGameServersRequest, opts ...grpc.CallOption) (*ListGameServersResponse, error)
	GameServerMapsLoaded(ctx context.Context, in *GameServerMapsLoadedRequest, opts ...grpc.CallOption) (*GameServerMapsLoadedResponse, error)
	RegisterGateway(ctx context.Context, in *RegisterGatewayRequest, opts ...grpc.CallOption) (*RegisterGatewayResponse, error)
	GatewaysForRealms(ctx context.Context, in *GatewaysForRealmsRequest, opts ...grpc.CallOption) (*GatewaysForRealmsResponse, error)
	ListGatewaysForRealm(ctx context.Context, in *ListGatewaysForRealmRequest, opts ...grpc.CallOption) (*ListGatewaysForRealmResponse, error)
}

type serversRegistryServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewServersRegistryServiceClient(cc grpc.ClientConnInterface) ServersRegistryServiceClient {
	return &serversRegistryServiceClient{cc}
}

func (c *serversRegistryServiceClient) RegisterGameServer(ctx context.Context, in *RegisterGameServerRequest, opts ...grpc.CallOption) (*RegisterGameServerResponse, error) {
	out := new(RegisterGameServerResponse)
	err := c.cc.Invoke(ctx, ServersRegistryService_RegisterGameServer_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serversRegistryServiceClient) AvailableGameServersForMapAndRealm(ctx context.Context, in *AvailableGameServersForMapAndRealmRequest, opts ...grpc.CallOption) (*AvailableGameServersForMapAndRealmResponse, error) {
	out := new(AvailableGameServersForMapAndRealmResponse)
	err := c.cc.Invoke(ctx, ServersRegistryService_AvailableGameServersForMapAndRealm_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serversRegistryServiceClient) RandomGameServerForRealm(ctx context.Context, in *RandomGameServerForRealmRequest, opts ...grpc.CallOption) (*RandomGameServerForRealmResponse, error) {
	out := new(RandomGameServerForRealmResponse)
	err := c.cc.Invoke(ctx, ServersRegistryService_RandomGameServerForRealm_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serversRegistryServiceClient) ListGameServersForRealm(ctx context.Context, in *ListGameServersForRealmRequest, opts ...grpc.CallOption) (*ListGameServersResponse, error) {
	out := new(ListGameServersResponse)
	err := c.cc.Invoke(ctx, ServersRegistryService_ListGameServersForRealm_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serversRegistryServiceClient) ListAllGameServers(ctx context.Context, in *ListAllGameServersRequest, opts ...grpc.CallOption) (*ListGameServersResponse, error) {
	out := new(ListGameServersResponse)
	err := c.cc.Invoke(ctx, ServersRegistryService_ListAllGameServers_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serversRegistryServiceClient) GameServerMapsLoaded(ctx context.Context, in *GameServerMapsLoadedRequest, opts ...grpc.CallOption) (*GameServerMapsLoadedResponse, error) {
	out := new(GameServerMapsLoadedResponse)
	err := c.cc.Invoke(ctx, ServersRegistryService_GameServerMapsLoaded_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serversRegistryServiceClient) RegisterGateway(ctx context.Context, in *RegisterGatewayRequest, opts ...grpc.CallOption) (*RegisterGatewayResponse, error) {
	out := new(RegisterGatewayResponse)
	err := c.cc.Invoke(ctx, ServersRegistryService_RegisterGateway_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serversRegistryServiceClient) GatewaysForRealms(ctx context.Context, in *GatewaysForRealmsRequest, opts ...grpc.CallOption) (*GatewaysForRealmsResponse, error) {
	out := new(GatewaysForRealmsResponse)
	err := c.cc.Invoke(ctx, ServersRegistryService_GatewaysForRealms_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serversRegistryServiceClient) ListGatewaysForRealm(ctx context.Context, in *ListGatewaysForRealmRequest, opts ...grpc.CallOption) (*ListGatewaysForRealmResponse, error) {
	out := new(ListGatewaysForRealmResponse)
	err := c.cc.Invoke(ctx, ServersRegistryService_ListGatewaysForRealm_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ServersRegistryServiceServer is the server API for ServersRegistryService service.
// All implementations must embed UnimplementedServersRegistryServiceServer
// for forward compatibility
type ServersRegistryServiceServer interface {
	RegisterGameServer(context.Context, *RegisterGameServerRequest) (*RegisterGameServerResponse, error)
	AvailableGameServersForMapAndRealm(context.Context, *AvailableGameServersForMapAndRealmRequest) (*AvailableGameServersForMapAndRealmResponse, error)
	RandomGameServerForRealm(context.Context, *RandomGameServerForRealmRequest) (*RandomGameServerForRealmResponse, error)
	ListGameServersForRealm(context.Context, *ListGameServersForRealmRequest) (*ListGameServersResponse, error)
	ListAllGameServers(context.Context, *ListAllGameServersRequest) (*ListGameServersResponse, error)
	GameServerMapsLoaded(context.Context, *GameServerMapsLoadedRequest) (*GameServerMapsLoadedResponse, error)
	RegisterGateway(context.Context, *RegisterGatewayRequest) (*RegisterGatewayResponse, error)
	GatewaysForRealms(context.Context, *GatewaysForRealmsRequest) (*GatewaysForRealmsResponse, error)
	ListGatewaysForRealm(context.Context, *ListGatewaysForRealmRequest) (*ListGatewaysForRealmResponse, error)
	mustEmbedUnimplementedServersRegistryServiceServer()
}

// UnimplementedServersRegistryServiceServer must be embedded to have forward compatible implementations.
type UnimplementedServersRegistryServiceServer struct {
}

func (UnimplementedServersRegistryServiceServer) RegisterGameServer(context.Context, *RegisterGameServerRequest) (*RegisterGameServerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RegisterGameServer not implemented")
}
func (UnimplementedServersRegistryServiceServer) AvailableGameServersForMapAndRealm(context.Context, *AvailableGameServersForMapAndRealmRequest) (*AvailableGameServersForMapAndRealmResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AvailableGameServersForMapAndRealm not implemented")
}
func (UnimplementedServersRegistryServiceServer) RandomGameServerForRealm(context.Context, *RandomGameServerForRealmRequest) (*RandomGameServerForRealmResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RandomGameServerForRealm not implemented")
}
func (UnimplementedServersRegistryServiceServer) ListGameServersForRealm(context.Context, *ListGameServersForRealmRequest) (*ListGameServersResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListGameServersForRealm not implemented")
}
func (UnimplementedServersRegistryServiceServer) ListAllGameServers(context.Context, *ListAllGameServersRequest) (*ListGameServersResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListAllGameServers not implemented")
}
func (UnimplementedServersRegistryServiceServer) GameServerMapsLoaded(context.Context, *GameServerMapsLoadedRequest) (*GameServerMapsLoadedResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GameServerMapsLoaded not implemented")
}
func (UnimplementedServersRegistryServiceServer) RegisterGateway(context.Context, *RegisterGatewayRequest) (*RegisterGatewayResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RegisterGateway not implemented")
}
func (UnimplementedServersRegistryServiceServer) GatewaysForRealms(context.Context, *GatewaysForRealmsRequest) (*GatewaysForRealmsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GatewaysForRealms not implemented")
}
func (UnimplementedServersRegistryServiceServer) ListGatewaysForRealm(context.Context, *ListGatewaysForRealmRequest) (*ListGatewaysForRealmResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListGatewaysForRealm not implemented")
}
func (UnimplementedServersRegistryServiceServer) mustEmbedUnimplementedServersRegistryServiceServer() {
}

// UnsafeServersRegistryServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ServersRegistryServiceServer will
// result in compilation errors.
type UnsafeServersRegistryServiceServer interface {
	mustEmbedUnimplementedServersRegistryServiceServer()
}

func RegisterServersRegistryServiceServer(s grpc.ServiceRegistrar, srv ServersRegistryServiceServer) {
	s.RegisterService(&ServersRegistryService_ServiceDesc, srv)
}

func _ServersRegistryService_RegisterGameServer_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RegisterGameServerRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServersRegistryServiceServer).RegisterGameServer(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ServersRegistryService_RegisterGameServer_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServersRegistryServiceServer).RegisterGameServer(ctx, req.(*RegisterGameServerRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServersRegistryService_AvailableGameServersForMapAndRealm_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AvailableGameServersForMapAndRealmRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServersRegistryServiceServer).AvailableGameServersForMapAndRealm(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ServersRegistryService_AvailableGameServersForMapAndRealm_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServersRegistryServiceServer).AvailableGameServersForMapAndRealm(ctx, req.(*AvailableGameServersForMapAndRealmRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServersRegistryService_RandomGameServerForRealm_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RandomGameServerForRealmRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServersRegistryServiceServer).RandomGameServerForRealm(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ServersRegistryService_RandomGameServerForRealm_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServersRegistryServiceServer).RandomGameServerForRealm(ctx, req.(*RandomGameServerForRealmRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServersRegistryService_ListGameServersForRealm_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListGameServersForRealmRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServersRegistryServiceServer).ListGameServersForRealm(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ServersRegistryService_ListGameServersForRealm_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServersRegistryServiceServer).ListGameServersForRealm(ctx, req.(*ListGameServersForRealmRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServersRegistryService_ListAllGameServers_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListAllGameServersRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServersRegistryServiceServer).ListAllGameServers(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ServersRegistryService_ListAllGameServers_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServersRegistryServiceServer).ListAllGameServers(ctx, req.(*ListAllGameServersRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServersRegistryService_GameServerMapsLoaded_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GameServerMapsLoadedRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServersRegistryServiceServer).GameServerMapsLoaded(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ServersRegistryService_GameServerMapsLoaded_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServersRegistryServiceServer).GameServerMapsLoaded(ctx, req.(*GameServerMapsLoadedRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServersRegistryService_RegisterGateway_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RegisterGatewayRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServersRegistryServiceServer).RegisterGateway(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ServersRegistryService_RegisterGateway_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServersRegistryServiceServer).RegisterGateway(ctx, req.(*RegisterGatewayRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServersRegistryService_GatewaysForRealms_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GatewaysForRealmsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServersRegistryServiceServer).GatewaysForRealms(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ServersRegistryService_GatewaysForRealms_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServersRegistryServiceServer).GatewaysForRealms(ctx, req.(*GatewaysForRealmsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ServersRegistryService_ListGatewaysForRealm_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListGatewaysForRealmRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServersRegistryServiceServer).ListGatewaysForRealm(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ServersRegistryService_ListGatewaysForRealm_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServersRegistryServiceServer).ListGatewaysForRealm(ctx, req.(*ListGatewaysForRealmRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// ServersRegistryService_ServiceDesc is the grpc.ServiceDesc for ServersRegistryService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var ServersRegistryService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "v1.ServersRegistryService",
	HandlerType: (*ServersRegistryServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "RegisterGameServer",
			Handler:    _ServersRegistryService_RegisterGameServer_Handler,
		},
		{
			MethodName: "AvailableGameServersForMapAndRealm",
			Handler:    _ServersRegistryService_AvailableGameServersForMapAndRealm_Handler,
		},
		{
			MethodName: "RandomGameServerForRealm",
			Handler:    _ServersRegistryService_RandomGameServerForRealm_Handler,
		},
		{
			MethodName: "ListGameServersForRealm",
			Handler:    _ServersRegistryService_ListGameServersForRealm_Handler,
		},
		{
			MethodName: "ListAllGameServers",
			Handler:    _ServersRegistryService_ListAllGameServers_Handler,
		},
		{
			MethodName: "GameServerMapsLoaded",
			Handler:    _ServersRegistryService_GameServerMapsLoaded_Handler,
		},
		{
			MethodName: "RegisterGateway",
			Handler:    _ServersRegistryService_RegisterGateway_Handler,
		},
		{
			MethodName: "GatewaysForRealms",
			Handler:    _ServersRegistryService_GatewaysForRealms_Handler,
		},
		{
			MethodName: "ListGatewaysForRealm",
			Handler:    _ServersRegistryService_ListGatewaysForRealm_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "registry.proto",
}
