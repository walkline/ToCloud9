// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.32.0
// 	protoc        v3.20.3
// source: chat.proto

package pb

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type SendWhisperMessageResponse_Status int32

const (
	SendWhisperMessageResponse_Ok                SendWhisperMessageResponse_Status = 0
	SendWhisperMessageResponse_CharacterNotFound SendWhisperMessageResponse_Status = 2
)

// Enum value maps for SendWhisperMessageResponse_Status.
var (
	SendWhisperMessageResponse_Status_name = map[int32]string{
		0: "Ok",
		2: "CharacterNotFound",
	}
	SendWhisperMessageResponse_Status_value = map[string]int32{
		"Ok":                0,
		"CharacterNotFound": 2,
	}
)

func (x SendWhisperMessageResponse_Status) Enum() *SendWhisperMessageResponse_Status {
	p := new(SendWhisperMessageResponse_Status)
	*p = x
	return p
}

func (x SendWhisperMessageResponse_Status) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (SendWhisperMessageResponse_Status) Descriptor() protoreflect.EnumDescriptor {
	return file_chat_proto_enumTypes[0].Descriptor()
}

func (SendWhisperMessageResponse_Status) Type() protoreflect.EnumType {
	return &file_chat_proto_enumTypes[0]
}

func (x SendWhisperMessageResponse_Status) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use SendWhisperMessageResponse_Status.Descriptor instead.
func (SendWhisperMessageResponse_Status) EnumDescriptor() ([]byte, []int) {
	return file_chat_proto_rawDescGZIP(), []int{1, 0}
}

type SendWhisperMessageRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Api          string `protobuf:"bytes,1,opt,name=api,proto3" json:"api,omitempty"`
	RealmID      uint32 `protobuf:"varint,2,opt,name=realmID,proto3" json:"realmID,omitempty"`
	SenderGUID   uint64 `protobuf:"varint,3,opt,name=senderGUID,proto3" json:"senderGUID,omitempty"`
	SenderName   string `protobuf:"bytes,4,opt,name=senderName,proto3" json:"senderName,omitempty"`
	SenderRace   uint32 `protobuf:"varint,5,opt,name=senderRace,proto3" json:"senderRace,omitempty"`
	Language     uint32 `protobuf:"varint,6,opt,name=language,proto3" json:"language,omitempty"`
	ReceiverName string `protobuf:"bytes,7,opt,name=receiverName,proto3" json:"receiverName,omitempty"`
	Msg          string `protobuf:"bytes,8,opt,name=msg,proto3" json:"msg,omitempty"`
}

func (x *SendWhisperMessageRequest) Reset() {
	*x = SendWhisperMessageRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_chat_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SendWhisperMessageRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SendWhisperMessageRequest) ProtoMessage() {}

func (x *SendWhisperMessageRequest) ProtoReflect() protoreflect.Message {
	mi := &file_chat_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SendWhisperMessageRequest.ProtoReflect.Descriptor instead.
func (*SendWhisperMessageRequest) Descriptor() ([]byte, []int) {
	return file_chat_proto_rawDescGZIP(), []int{0}
}

func (x *SendWhisperMessageRequest) GetApi() string {
	if x != nil {
		return x.Api
	}
	return ""
}

func (x *SendWhisperMessageRequest) GetRealmID() uint32 {
	if x != nil {
		return x.RealmID
	}
	return 0
}

func (x *SendWhisperMessageRequest) GetSenderGUID() uint64 {
	if x != nil {
		return x.SenderGUID
	}
	return 0
}

func (x *SendWhisperMessageRequest) GetSenderName() string {
	if x != nil {
		return x.SenderName
	}
	return ""
}

func (x *SendWhisperMessageRequest) GetSenderRace() uint32 {
	if x != nil {
		return x.SenderRace
	}
	return 0
}

func (x *SendWhisperMessageRequest) GetLanguage() uint32 {
	if x != nil {
		return x.Language
	}
	return 0
}

func (x *SendWhisperMessageRequest) GetReceiverName() string {
	if x != nil {
		return x.ReceiverName
	}
	return ""
}

func (x *SendWhisperMessageRequest) GetMsg() string {
	if x != nil {
		return x.Msg
	}
	return ""
}

type SendWhisperMessageResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Api          string                            `protobuf:"bytes,1,opt,name=api,proto3" json:"api,omitempty"`
	Status       SendWhisperMessageResponse_Status `protobuf:"varint,2,opt,name=status,proto3,enum=v1.SendWhisperMessageResponse_Status" json:"status,omitempty"`
	ReceiverGUID uint64                            `protobuf:"varint,3,opt,name=receiverGUID,proto3" json:"receiverGUID,omitempty"`
}

func (x *SendWhisperMessageResponse) Reset() {
	*x = SendWhisperMessageResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_chat_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SendWhisperMessageResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SendWhisperMessageResponse) ProtoMessage() {}

func (x *SendWhisperMessageResponse) ProtoReflect() protoreflect.Message {
	mi := &file_chat_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SendWhisperMessageResponse.ProtoReflect.Descriptor instead.
func (*SendWhisperMessageResponse) Descriptor() ([]byte, []int) {
	return file_chat_proto_rawDescGZIP(), []int{1}
}

func (x *SendWhisperMessageResponse) GetApi() string {
	if x != nil {
		return x.Api
	}
	return ""
}

func (x *SendWhisperMessageResponse) GetStatus() SendWhisperMessageResponse_Status {
	if x != nil {
		return x.Status
	}
	return SendWhisperMessageResponse_Ok
}

func (x *SendWhisperMessageResponse) GetReceiverGUID() uint64 {
	if x != nil {
		return x.ReceiverGUID
	}
	return 0
}

var File_chat_proto protoreflect.FileDescriptor

var file_chat_proto_rawDesc = []byte{
	0x0a, 0x0a, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x02, 0x76, 0x31,
	0x22, 0xf9, 0x01, 0x0a, 0x19, 0x53, 0x65, 0x6e, 0x64, 0x57, 0x68, 0x69, 0x73, 0x70, 0x65, 0x72,
	0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x10,
	0x0a, 0x03, 0x61, 0x70, 0x69, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x61, 0x70, 0x69,
	0x12, 0x18, 0x0a, 0x07, 0x72, 0x65, 0x61, 0x6c, 0x6d, 0x49, 0x44, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x0d, 0x52, 0x07, 0x72, 0x65, 0x61, 0x6c, 0x6d, 0x49, 0x44, 0x12, 0x1e, 0x0a, 0x0a, 0x73, 0x65,
	0x6e, 0x64, 0x65, 0x72, 0x47, 0x55, 0x49, 0x44, 0x18, 0x03, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0a,
	0x73, 0x65, 0x6e, 0x64, 0x65, 0x72, 0x47, 0x55, 0x49, 0x44, 0x12, 0x1e, 0x0a, 0x0a, 0x73, 0x65,
	0x6e, 0x64, 0x65, 0x72, 0x4e, 0x61, 0x6d, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a,
	0x73, 0x65, 0x6e, 0x64, 0x65, 0x72, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x1e, 0x0a, 0x0a, 0x73, 0x65,
	0x6e, 0x64, 0x65, 0x72, 0x52, 0x61, 0x63, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x0a,
	0x73, 0x65, 0x6e, 0x64, 0x65, 0x72, 0x52, 0x61, 0x63, 0x65, 0x12, 0x1a, 0x0a, 0x08, 0x6c, 0x61,
	0x6e, 0x67, 0x75, 0x61, 0x67, 0x65, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x08, 0x6c, 0x61,
	0x6e, 0x67, 0x75, 0x61, 0x67, 0x65, 0x12, 0x22, 0x0a, 0x0c, 0x72, 0x65, 0x63, 0x65, 0x69, 0x76,
	0x65, 0x72, 0x4e, 0x61, 0x6d, 0x65, 0x18, 0x07, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0c, 0x72, 0x65,
	0x63, 0x65, 0x69, 0x76, 0x65, 0x72, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x10, 0x0a, 0x03, 0x6d, 0x73,
	0x67, 0x18, 0x08, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6d, 0x73, 0x67, 0x22, 0xba, 0x01, 0x0a,
	0x1a, 0x53, 0x65, 0x6e, 0x64, 0x57, 0x68, 0x69, 0x73, 0x70, 0x65, 0x72, 0x4d, 0x65, 0x73, 0x73,
	0x61, 0x67, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x10, 0x0a, 0x03, 0x61,
	0x70, 0x69, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x61, 0x70, 0x69, 0x12, 0x3d, 0x0a,
	0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x25, 0x2e,
	0x76, 0x31, 0x2e, 0x53, 0x65, 0x6e, 0x64, 0x57, 0x68, 0x69, 0x73, 0x70, 0x65, 0x72, 0x4d, 0x65,
	0x73, 0x73, 0x61, 0x67, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x2e, 0x53, 0x74,
	0x61, 0x74, 0x75, 0x73, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x22, 0x0a, 0x0c,
	0x72, 0x65, 0x63, 0x65, 0x69, 0x76, 0x65, 0x72, 0x47, 0x55, 0x49, 0x44, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x04, 0x52, 0x0c, 0x72, 0x65, 0x63, 0x65, 0x69, 0x76, 0x65, 0x72, 0x47, 0x55, 0x49, 0x44,
	0x22, 0x27, 0x0a, 0x06, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x06, 0x0a, 0x02, 0x4f, 0x6b,
	0x10, 0x00, 0x12, 0x15, 0x0a, 0x11, 0x43, 0x68, 0x61, 0x72, 0x61, 0x63, 0x74, 0x65, 0x72, 0x4e,
	0x6f, 0x74, 0x46, 0x6f, 0x75, 0x6e, 0x64, 0x10, 0x02, 0x32, 0x62, 0x0a, 0x0b, 0x43, 0x68, 0x61,
	0x74, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x53, 0x0a, 0x12, 0x53, 0x65, 0x6e, 0x64,
	0x57, 0x68, 0x69, 0x73, 0x70, 0x65, 0x72, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x1d,
	0x2e, 0x76, 0x31, 0x2e, 0x53, 0x65, 0x6e, 0x64, 0x57, 0x68, 0x69, 0x73, 0x70, 0x65, 0x72, 0x4d,
	0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1e, 0x2e,
	0x76, 0x31, 0x2e, 0x53, 0x65, 0x6e, 0x64, 0x57, 0x68, 0x69, 0x73, 0x70, 0x65, 0x72, 0x4d, 0x65,
	0x73, 0x73, 0x61, 0x67, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x42, 0x0d, 0x5a,
	0x0b, 0x67, 0x65, 0x6e, 0x2f, 0x63, 0x68, 0x61, 0x74, 0x2f, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_chat_proto_rawDescOnce sync.Once
	file_chat_proto_rawDescData = file_chat_proto_rawDesc
)

func file_chat_proto_rawDescGZIP() []byte {
	file_chat_proto_rawDescOnce.Do(func() {
		file_chat_proto_rawDescData = protoimpl.X.CompressGZIP(file_chat_proto_rawDescData)
	})
	return file_chat_proto_rawDescData
}

var file_chat_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_chat_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_chat_proto_goTypes = []interface{}{
	(SendWhisperMessageResponse_Status)(0), // 0: v1.SendWhisperMessageResponse.Status
	(*SendWhisperMessageRequest)(nil),      // 1: v1.SendWhisperMessageRequest
	(*SendWhisperMessageResponse)(nil),     // 2: v1.SendWhisperMessageResponse
}
var file_chat_proto_depIdxs = []int32{
	0, // 0: v1.SendWhisperMessageResponse.status:type_name -> v1.SendWhisperMessageResponse.Status
	1, // 1: v1.ChatService.SendWhisperMessage:input_type -> v1.SendWhisperMessageRequest
	2, // 2: v1.ChatService.SendWhisperMessage:output_type -> v1.SendWhisperMessageResponse
	2, // [2:3] is the sub-list for method output_type
	1, // [1:2] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_chat_proto_init() }
func file_chat_proto_init() {
	if File_chat_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_chat_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SendWhisperMessageRequest); i {
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
		file_chat_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SendWhisperMessageResponse); i {
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
			RawDescriptor: file_chat_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_chat_proto_goTypes,
		DependencyIndexes: file_chat_proto_depIdxs,
		EnumInfos:         file_chat_proto_enumTypes,
		MessageInfos:      file_chat_proto_msgTypes,
	}.Build()
	File_chat_proto = out.File
	file_chat_proto_rawDesc = nil
	file_chat_proto_goTypes = nil
	file_chat_proto_depIdxs = nil
}
