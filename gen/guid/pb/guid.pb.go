// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1-devel
// 	protoc        v4.23.4
// source: guid.proto

package pb

import (
	context "context"
	reflect "reflect"
	sync "sync"

	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type GuidType int32

const (
	GuidType_Character GuidType = 0
	GuidType_Item      GuidType = 1
)

// Enum value maps for GuidType.
var (
	GuidType_name = map[int32]string{
		0: "Character",
		1: "Item",
	}
	GuidType_value = map[string]int32{
		"Character": 0,
		"Item":      1,
	}
)

func (x GuidType) Enum() *GuidType {
	p := new(GuidType)
	*p = x
	return p
}

func (x GuidType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (GuidType) Descriptor() protoreflect.EnumDescriptor {
	return file_guid_proto_enumTypes[0].Descriptor()
}

func (GuidType) Type() protoreflect.EnumType {
	return &file_guid_proto_enumTypes[0]
}

func (x GuidType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use GuidType.Descriptor instead.
func (GuidType) EnumDescriptor() ([]byte, []int) {
	return file_guid_proto_rawDescGZIP(), []int{0}
}

type GetGUIDPoolRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Api             string   `protobuf:"bytes,1,opt,name=api,proto3" json:"api,omitempty"`
	RealmID         uint32   `protobuf:"varint,2,opt,name=realmID,proto3" json:"realmID,omitempty"`
	GuidType        GuidType `protobuf:"varint,3,opt,name=guidType,proto3,enum=v1.GuidType" json:"guidType,omitempty"`
	DesiredPoolSize uint64   `protobuf:"varint,4,opt,name=desiredPoolSize,proto3" json:"desiredPoolSize,omitempty"`
}

func (x *GetGUIDPoolRequest) Reset() {
	*x = GetGUIDPoolRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_guid_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetGUIDPoolRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetGUIDPoolRequest) ProtoMessage() {}

func (x *GetGUIDPoolRequest) ProtoReflect() protoreflect.Message {
	mi := &file_guid_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetGUIDPoolRequest.ProtoReflect.Descriptor instead.
func (*GetGUIDPoolRequest) Descriptor() ([]byte, []int) {
	return file_guid_proto_rawDescGZIP(), []int{0}
}

func (x *GetGUIDPoolRequest) GetApi() string {
	if x != nil {
		return x.Api
	}
	return ""
}

func (x *GetGUIDPoolRequest) GetRealmID() uint32 {
	if x != nil {
		return x.RealmID
	}
	return 0
}

func (x *GetGUIDPoolRequest) GetGuidType() GuidType {
	if x != nil {
		return x.GuidType
	}
	return GuidType_Character
}

func (x *GetGUIDPoolRequest) GetDesiredPoolSize() uint64 {
	if x != nil {
		return x.DesiredPoolSize
	}
	return 0
}

type GetGUIDPoolRequestResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Api          string                                     `protobuf:"bytes,1,opt,name=api,proto3" json:"api,omitempty"`
	ReceiverGUID []*GetGUIDPoolRequestResponse_GuidDiapason `protobuf:"bytes,2,rep,name=receiverGUID,proto3" json:"receiverGUID,omitempty"`
}

func (x *GetGUIDPoolRequestResponse) Reset() {
	*x = GetGUIDPoolRequestResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_guid_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetGUIDPoolRequestResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetGUIDPoolRequestResponse) ProtoMessage() {}

func (x *GetGUIDPoolRequestResponse) ProtoReflect() protoreflect.Message {
	mi := &file_guid_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetGUIDPoolRequestResponse.ProtoReflect.Descriptor instead.
func (*GetGUIDPoolRequestResponse) Descriptor() ([]byte, []int) {
	return file_guid_proto_rawDescGZIP(), []int{1}
}

func (x *GetGUIDPoolRequestResponse) GetApi() string {
	if x != nil {
		return x.Api
	}
	return ""
}

func (x *GetGUIDPoolRequestResponse) GetReceiverGUID() []*GetGUIDPoolRequestResponse_GuidDiapason {
	if x != nil {
		return x.ReceiverGUID
	}
	return nil
}

type GetGUIDPoolRequestResponse_GuidDiapason struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Start uint64 `protobuf:"varint,1,opt,name=start,proto3" json:"start,omitempty"`
	End   uint64 `protobuf:"varint,2,opt,name=end,proto3" json:"end,omitempty"`
}

func (x *GetGUIDPoolRequestResponse_GuidDiapason) Reset() {
	*x = GetGUIDPoolRequestResponse_GuidDiapason{}
	if protoimpl.UnsafeEnabled {
		mi := &file_guid_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetGUIDPoolRequestResponse_GuidDiapason) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetGUIDPoolRequestResponse_GuidDiapason) ProtoMessage() {}

func (x *GetGUIDPoolRequestResponse_GuidDiapason) ProtoReflect() protoreflect.Message {
	mi := &file_guid_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetGUIDPoolRequestResponse_GuidDiapason.ProtoReflect.Descriptor instead.
func (*GetGUIDPoolRequestResponse_GuidDiapason) Descriptor() ([]byte, []int) {
	return file_guid_proto_rawDescGZIP(), []int{1, 0}
}

func (x *GetGUIDPoolRequestResponse_GuidDiapason) GetStart() uint64 {
	if x != nil {
		return x.Start
	}
	return 0
}

func (x *GetGUIDPoolRequestResponse_GuidDiapason) GetEnd() uint64 {
	if x != nil {
		return x.End
	}
	return 0
}

var File_guid_proto protoreflect.FileDescriptor

var file_guid_proto_rawDesc = []byte{
	0x0a, 0x0a, 0x67, 0x75, 0x69, 0x64, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x02, 0x76, 0x31,
	0x22, 0x94, 0x01, 0x0a, 0x12, 0x47, 0x65, 0x74, 0x47, 0x55, 0x49, 0x44, 0x50, 0x6f, 0x6f, 0x6c,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x10, 0x0a, 0x03, 0x61, 0x70, 0x69, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x61, 0x70, 0x69, 0x12, 0x18, 0x0a, 0x07, 0x72, 0x65, 0x61,
	0x6c, 0x6d, 0x49, 0x44, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x07, 0x72, 0x65, 0x61, 0x6c,
	0x6d, 0x49, 0x44, 0x12, 0x28, 0x0a, 0x08, 0x67, 0x75, 0x69, 0x64, 0x54, 0x79, 0x70, 0x65, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x0c, 0x2e, 0x76, 0x31, 0x2e, 0x47, 0x75, 0x69, 0x64, 0x54,
	0x79, 0x70, 0x65, 0x52, 0x08, 0x67, 0x75, 0x69, 0x64, 0x54, 0x79, 0x70, 0x65, 0x12, 0x28, 0x0a,
	0x0f, 0x64, 0x65, 0x73, 0x69, 0x72, 0x65, 0x64, 0x50, 0x6f, 0x6f, 0x6c, 0x53, 0x69, 0x7a, 0x65,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0f, 0x64, 0x65, 0x73, 0x69, 0x72, 0x65, 0x64, 0x50,
	0x6f, 0x6f, 0x6c, 0x53, 0x69, 0x7a, 0x65, 0x22, 0xb7, 0x01, 0x0a, 0x1a, 0x47, 0x65, 0x74, 0x47,
	0x55, 0x49, 0x44, 0x50, 0x6f, 0x6f, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x10, 0x0a, 0x03, 0x61, 0x70, 0x69, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x03, 0x61, 0x70, 0x69, 0x12, 0x4f, 0x0a, 0x0c, 0x72, 0x65, 0x63, 0x65,
	0x69, 0x76, 0x65, 0x72, 0x47, 0x55, 0x49, 0x44, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x2b,
	0x2e, 0x76, 0x31, 0x2e, 0x47, 0x65, 0x74, 0x47, 0x55, 0x49, 0x44, 0x50, 0x6f, 0x6f, 0x6c, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x2e, 0x47,
	0x75, 0x69, 0x64, 0x44, 0x69, 0x61, 0x70, 0x61, 0x73, 0x6f, 0x6e, 0x52, 0x0c, 0x72, 0x65, 0x63,
	0x65, 0x69, 0x76, 0x65, 0x72, 0x47, 0x55, 0x49, 0x44, 0x1a, 0x36, 0x0a, 0x0c, 0x47, 0x75, 0x69,
	0x64, 0x44, 0x69, 0x61, 0x70, 0x61, 0x73, 0x6f, 0x6e, 0x12, 0x14, 0x0a, 0x05, 0x73, 0x74, 0x61,
	0x72, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x05, 0x73, 0x74, 0x61, 0x72, 0x74, 0x12,
	0x10, 0x0a, 0x03, 0x65, 0x6e, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x04, 0x52, 0x03, 0x65, 0x6e,
	0x64, 0x2a, 0x23, 0x0a, 0x08, 0x47, 0x75, 0x69, 0x64, 0x54, 0x79, 0x70, 0x65, 0x12, 0x0d, 0x0a,
	0x09, 0x43, 0x68, 0x61, 0x72, 0x61, 0x63, 0x74, 0x65, 0x72, 0x10, 0x00, 0x12, 0x08, 0x0a, 0x04,
	0x49, 0x74, 0x65, 0x6d, 0x10, 0x01, 0x32, 0x54, 0x0a, 0x0b, 0x47, 0x75, 0x69, 0x64, 0x53, 0x65,
	0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x45, 0x0a, 0x0b, 0x47, 0x65, 0x74, 0x47, 0x55, 0x49, 0x44,
	0x50, 0x6f, 0x6f, 0x6c, 0x12, 0x16, 0x2e, 0x76, 0x31, 0x2e, 0x47, 0x65, 0x74, 0x47, 0x55, 0x49,
	0x44, 0x50, 0x6f, 0x6f, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1e, 0x2e, 0x76,
	0x31, 0x2e, 0x47, 0x65, 0x74, 0x47, 0x55, 0x49, 0x44, 0x50, 0x6f, 0x6f, 0x6c, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x42, 0x0d, 0x5a, 0x0b,
	0x67, 0x65, 0x6e, 0x2f, 0x67, 0x75, 0x69, 0x64, 0x2f, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x33,
}

var (
	file_guid_proto_rawDescOnce sync.Once
	file_guid_proto_rawDescData = file_guid_proto_rawDesc
)

func file_guid_proto_rawDescGZIP() []byte {
	file_guid_proto_rawDescOnce.Do(func() {
		file_guid_proto_rawDescData = protoimpl.X.CompressGZIP(file_guid_proto_rawDescData)
	})
	return file_guid_proto_rawDescData
}

var file_guid_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_guid_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_guid_proto_goTypes = []interface{}{
	(GuidType)(0),                                   // 0: v1.GuidType
	(*GetGUIDPoolRequest)(nil),                      // 1: v1.GetGUIDPoolRequest
	(*GetGUIDPoolRequestResponse)(nil),              // 2: v1.GetGUIDPoolRequestResponse
	(*GetGUIDPoolRequestResponse_GuidDiapason)(nil), // 3: v1.GetGUIDPoolRequestResponse.GuidDiapason
}
var file_guid_proto_depIdxs = []int32{
	0, // 0: v1.GetGUIDPoolRequest.guidType:type_name -> v1.GuidType
	3, // 1: v1.GetGUIDPoolRequestResponse.receiverGUID:type_name -> v1.GetGUIDPoolRequestResponse.GuidDiapason
	1, // 2: v1.GuidService.GetGUIDPool:input_type -> v1.GetGUIDPoolRequest
	2, // 3: v1.GuidService.GetGUIDPool:output_type -> v1.GetGUIDPoolRequestResponse
	3, // [3:4] is the sub-list for method output_type
	2, // [2:3] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_guid_proto_init() }
func file_guid_proto_init() {
	if File_guid_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_guid_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetGUIDPoolRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_guid_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetGUIDPoolRequestResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_guid_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetGUIDPoolRequestResponse_GuidDiapason); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_guid_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_guid_proto_goTypes,
		DependencyIndexes: file_guid_proto_depIdxs,
		EnumInfos:         file_guid_proto_enumTypes,
		MessageInfos:      file_guid_proto_msgTypes,
	}.Build()
	File_guid_proto = out.File
	file_guid_proto_rawDesc = nil
	file_guid_proto_goTypes = nil
	file_guid_proto_depIdxs = nil
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConnInterface

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion6

// GuidServiceClient is the client API for GuidService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type GuidServiceClient interface {
	GetGUIDPool(ctx context.Context, in *GetGUIDPoolRequest, opts ...grpc.CallOption) (*GetGUIDPoolRequestResponse, error)
}

type guidServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewGuidServiceClient(cc grpc.ClientConnInterface) GuidServiceClient {
	return &guidServiceClient{cc}
}

func (c *guidServiceClient) GetGUIDPool(ctx context.Context, in *GetGUIDPoolRequest, opts ...grpc.CallOption) (*GetGUIDPoolRequestResponse, error) {
	out := new(GetGUIDPoolRequestResponse)
	err := c.cc.Invoke(ctx, "/v1.GuidService/GetGUIDPool", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GuidServiceServer is the server API for GuidService service.
type GuidServiceServer interface {
	GetGUIDPool(context.Context, *GetGUIDPoolRequest) (*GetGUIDPoolRequestResponse, error)
}

// UnimplementedGuidServiceServer can be embedded to have forward compatible implementations.
type UnimplementedGuidServiceServer struct {
}

func (*UnimplementedGuidServiceServer) GetGUIDPool(context.Context, *GetGUIDPoolRequest) (*GetGUIDPoolRequestResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetGUIDPool not implemented")
}

func RegisterGuidServiceServer(s *grpc.Server, srv GuidServiceServer) {
	s.RegisterService(&_GuidService_serviceDesc, srv)
}

func _GuidService_GetGUIDPool_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetGUIDPoolRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GuidServiceServer).GetGUIDPool(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/v1.GuidService/GetGUIDPool",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GuidServiceServer).GetGUIDPool(ctx, req.(*GetGUIDPoolRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _GuidService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "v1.GuidService",
	HandlerType: (*GuidServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetGUIDPool",
			Handler:    _GuidService_GetGUIDPool_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "guid.proto",
}
