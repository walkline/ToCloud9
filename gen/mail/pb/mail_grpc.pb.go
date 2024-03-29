// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v3.20.3
// source: mail.proto

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
	MailService_Send_FullMethodName                = "/v1.MailService/Send"
	MailService_MarkAsReadForPlayer_FullMethodName = "/v1.MailService/MarkAsReadForPlayer"
	MailService_RemoveMailItem_FullMethodName      = "/v1.MailService/RemoveMailItem"
	MailService_RemoveMailMoney_FullMethodName     = "/v1.MailService/RemoveMailMoney"
	MailService_MailByID_FullMethodName            = "/v1.MailService/MailByID"
	MailService_MailsForPlayer_FullMethodName      = "/v1.MailService/MailsForPlayer"
	MailService_DeleteMail_FullMethodName          = "/v1.MailService/DeleteMail"
)

// MailServiceClient is the client API for MailService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type MailServiceClient interface {
	Send(ctx context.Context, in *SendRequest, opts ...grpc.CallOption) (*SendResponse, error)
	MarkAsReadForPlayer(ctx context.Context, in *MarkAsReadForPlayerRequest, opts ...grpc.CallOption) (*MarkAsReadForPlayerResponse, error)
	RemoveMailItem(ctx context.Context, in *RemoveMailItemRequest, opts ...grpc.CallOption) (*RemoveMailItemResponse, error)
	RemoveMailMoney(ctx context.Context, in *RemoveMailMoneyRequest, opts ...grpc.CallOption) (*RemoveMailMoneyResponse, error)
	MailByID(ctx context.Context, in *MailByIDRequest, opts ...grpc.CallOption) (*MailByIDResponse, error)
	MailsForPlayer(ctx context.Context, in *MailsForPlayerRequest, opts ...grpc.CallOption) (*MailsForPlayerResponse, error)
	DeleteMail(ctx context.Context, in *DeleteMailRequest, opts ...grpc.CallOption) (*DeleteMailResponse, error)
}

type mailServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewMailServiceClient(cc grpc.ClientConnInterface) MailServiceClient {
	return &mailServiceClient{cc}
}

func (c *mailServiceClient) Send(ctx context.Context, in *SendRequest, opts ...grpc.CallOption) (*SendResponse, error) {
	out := new(SendResponse)
	err := c.cc.Invoke(ctx, MailService_Send_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *mailServiceClient) MarkAsReadForPlayer(ctx context.Context, in *MarkAsReadForPlayerRequest, opts ...grpc.CallOption) (*MarkAsReadForPlayerResponse, error) {
	out := new(MarkAsReadForPlayerResponse)
	err := c.cc.Invoke(ctx, MailService_MarkAsReadForPlayer_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *mailServiceClient) RemoveMailItem(ctx context.Context, in *RemoveMailItemRequest, opts ...grpc.CallOption) (*RemoveMailItemResponse, error) {
	out := new(RemoveMailItemResponse)
	err := c.cc.Invoke(ctx, MailService_RemoveMailItem_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *mailServiceClient) RemoveMailMoney(ctx context.Context, in *RemoveMailMoneyRequest, opts ...grpc.CallOption) (*RemoveMailMoneyResponse, error) {
	out := new(RemoveMailMoneyResponse)
	err := c.cc.Invoke(ctx, MailService_RemoveMailMoney_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *mailServiceClient) MailByID(ctx context.Context, in *MailByIDRequest, opts ...grpc.CallOption) (*MailByIDResponse, error) {
	out := new(MailByIDResponse)
	err := c.cc.Invoke(ctx, MailService_MailByID_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *mailServiceClient) MailsForPlayer(ctx context.Context, in *MailsForPlayerRequest, opts ...grpc.CallOption) (*MailsForPlayerResponse, error) {
	out := new(MailsForPlayerResponse)
	err := c.cc.Invoke(ctx, MailService_MailsForPlayer_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *mailServiceClient) DeleteMail(ctx context.Context, in *DeleteMailRequest, opts ...grpc.CallOption) (*DeleteMailResponse, error) {
	out := new(DeleteMailResponse)
	err := c.cc.Invoke(ctx, MailService_DeleteMail_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// MailServiceServer is the server API for MailService service.
// All implementations must embed UnimplementedMailServiceServer
// for forward compatibility
type MailServiceServer interface {
	Send(context.Context, *SendRequest) (*SendResponse, error)
	MarkAsReadForPlayer(context.Context, *MarkAsReadForPlayerRequest) (*MarkAsReadForPlayerResponse, error)
	RemoveMailItem(context.Context, *RemoveMailItemRequest) (*RemoveMailItemResponse, error)
	RemoveMailMoney(context.Context, *RemoveMailMoneyRequest) (*RemoveMailMoneyResponse, error)
	MailByID(context.Context, *MailByIDRequest) (*MailByIDResponse, error)
	MailsForPlayer(context.Context, *MailsForPlayerRequest) (*MailsForPlayerResponse, error)
	DeleteMail(context.Context, *DeleteMailRequest) (*DeleteMailResponse, error)
	mustEmbedUnimplementedMailServiceServer()
}

// UnimplementedMailServiceServer must be embedded to have forward compatible implementations.
type UnimplementedMailServiceServer struct {
}

func (UnimplementedMailServiceServer) Send(context.Context, *SendRequest) (*SendResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Send not implemented")
}
func (UnimplementedMailServiceServer) MarkAsReadForPlayer(context.Context, *MarkAsReadForPlayerRequest) (*MarkAsReadForPlayerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method MarkAsReadForPlayer not implemented")
}
func (UnimplementedMailServiceServer) RemoveMailItem(context.Context, *RemoveMailItemRequest) (*RemoveMailItemResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RemoveMailItem not implemented")
}
func (UnimplementedMailServiceServer) RemoveMailMoney(context.Context, *RemoveMailMoneyRequest) (*RemoveMailMoneyResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RemoveMailMoney not implemented")
}
func (UnimplementedMailServiceServer) MailByID(context.Context, *MailByIDRequest) (*MailByIDResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method MailByID not implemented")
}
func (UnimplementedMailServiceServer) MailsForPlayer(context.Context, *MailsForPlayerRequest) (*MailsForPlayerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method MailsForPlayer not implemented")
}
func (UnimplementedMailServiceServer) DeleteMail(context.Context, *DeleteMailRequest) (*DeleteMailResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteMail not implemented")
}
func (UnimplementedMailServiceServer) mustEmbedUnimplementedMailServiceServer() {}

// UnsafeMailServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to MailServiceServer will
// result in compilation errors.
type UnsafeMailServiceServer interface {
	mustEmbedUnimplementedMailServiceServer()
}

func RegisterMailServiceServer(s grpc.ServiceRegistrar, srv MailServiceServer) {
	s.RegisterService(&MailService_ServiceDesc, srv)
}

func _MailService_Send_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SendRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MailServiceServer).Send(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MailService_Send_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MailServiceServer).Send(ctx, req.(*SendRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MailService_MarkAsReadForPlayer_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MarkAsReadForPlayerRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MailServiceServer).MarkAsReadForPlayer(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MailService_MarkAsReadForPlayer_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MailServiceServer).MarkAsReadForPlayer(ctx, req.(*MarkAsReadForPlayerRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MailService_RemoveMailItem_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RemoveMailItemRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MailServiceServer).RemoveMailItem(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MailService_RemoveMailItem_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MailServiceServer).RemoveMailItem(ctx, req.(*RemoveMailItemRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MailService_RemoveMailMoney_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RemoveMailMoneyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MailServiceServer).RemoveMailMoney(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MailService_RemoveMailMoney_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MailServiceServer).RemoveMailMoney(ctx, req.(*RemoveMailMoneyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MailService_MailByID_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MailByIDRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MailServiceServer).MailByID(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MailService_MailByID_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MailServiceServer).MailByID(ctx, req.(*MailByIDRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MailService_MailsForPlayer_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MailsForPlayerRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MailServiceServer).MailsForPlayer(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MailService_MailsForPlayer_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MailServiceServer).MailsForPlayer(ctx, req.(*MailsForPlayerRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MailService_DeleteMail_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteMailRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MailServiceServer).DeleteMail(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MailService_DeleteMail_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MailServiceServer).DeleteMail(ctx, req.(*DeleteMailRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// MailService_ServiceDesc is the grpc.ServiceDesc for MailService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var MailService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "v1.MailService",
	HandlerType: (*MailServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Send",
			Handler:    _MailService_Send_Handler,
		},
		{
			MethodName: "MarkAsReadForPlayer",
			Handler:    _MailService_MarkAsReadForPlayer_Handler,
		},
		{
			MethodName: "RemoveMailItem",
			Handler:    _MailService_RemoveMailItem_Handler,
		},
		{
			MethodName: "RemoveMailMoney",
			Handler:    _MailService_RemoveMailMoney_Handler,
		},
		{
			MethodName: "MailByID",
			Handler:    _MailService_MailByID_Handler,
		},
		{
			MethodName: "MailsForPlayer",
			Handler:    _MailService_MailsForPlayer_Handler,
		},
		{
			MethodName: "DeleteMail",
			Handler:    _MailService_DeleteMail_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "mail.proto",
}
