// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.0
// 	protoc        v3.15.6
// source: agent.proto

package agent

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

type ClientType int32

const (
	ClientType_LND_GRPC    ClientType = 0
	ClientType_LND_REST    ClientType = 1
	ClientType_C_LIGHTNING ClientType = 2
	ClientType_ECLAIR      ClientType = 3
)

// Enum value maps for ClientType.
var (
	ClientType_name = map[int32]string{
		0: "LND_GRPC",
		1: "LND_REST",
		2: "C_LIGHTNING",
		3: "ECLAIR",
	}
	ClientType_value = map[string]int32{
		"LND_GRPC":    0,
		"LND_REST":    1,
		"C_LIGHTNING": 2,
		"ECLAIR":      3,
	}
)

func (x ClientType) Enum() *ClientType {
	p := new(ClientType)
	*p = x
	return p
}

func (x ClientType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ClientType) Descriptor() protoreflect.EnumDescriptor {
	return file_agent_proto_enumTypes[0].Descriptor()
}

func (ClientType) Type() protoreflect.EnumType {
	return &file_agent_proto_enumTypes[0]
}

func (x ClientType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ClientType.Descriptor instead.
func (ClientType) EnumDescriptor() ([]byte, []int) {
	return file_agent_proto_rawDescGZIP(), []int{0}
}

type Empty struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *Empty) Reset() {
	*x = Empty{}
	if protoimpl.UnsafeEnabled {
		mi := &file_agent_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Empty) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Empty) ProtoMessage() {}

func (x *Empty) ProtoReflect() protoreflect.Message {
	mi := &file_agent_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Empty.ProtoReflect.Descriptor instead.
func (*Empty) Descriptor() ([]byte, []int) {
	return file_agent_proto_rawDescGZIP(), []int{0}
}

type TimestampResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Timestamp int64 `protobuf:"varint,1,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
}

func (x *TimestampResponse) Reset() {
	*x = TimestampResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_agent_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TimestampResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TimestampResponse) ProtoMessage() {}

func (x *TimestampResponse) ProtoReflect() protoreflect.Message {
	mi := &file_agent_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TimestampResponse.ProtoReflect.Descriptor instead.
func (*TimestampResponse) Descriptor() ([]byte, []int) {
	return file_agent_proto_rawDescGZIP(), []int{1}
}

func (x *TimestampResponse) GetTimestamp() int64 {
	if x != nil {
		return x.Timestamp
	}
	return 0
}

type CredentialsRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	PubKey     string     `protobuf:"bytes,1,opt,name=pub_key,json=pubKey,proto3" json:"pub_key,omitempty"`
	ClientType ClientType `protobuf:"varint,2,opt,name=client_type,json=clientType,proto3,enum=agent.ClientType" json:"client_type,omitempty"`
}

func (x *CredentialsRequest) Reset() {
	*x = CredentialsRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_agent_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CredentialsRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CredentialsRequest) ProtoMessage() {}

func (x *CredentialsRequest) ProtoReflect() protoreflect.Message {
	mi := &file_agent_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CredentialsRequest.ProtoReflect.Descriptor instead.
func (*CredentialsRequest) Descriptor() ([]byte, []int) {
	return file_agent_proto_rawDescGZIP(), []int{2}
}

func (x *CredentialsRequest) GetPubKey() string {
	if x != nil {
		return x.PubKey
	}
	return ""
}

func (x *CredentialsRequest) GetClientType() ClientType {
	if x != nil {
		return x.ClientType
	}
	return ClientType_LND_GRPC
}

type DataRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Timestamp int64  `protobuf:"varint,1,opt,name=timestamp,proto3" json:"timestamp,omitempty"` // timestamp in nanoseconds
	Data      string `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`            // Raw JSON serialized data (since it can come from various sources)
}

func (x *DataRequest) Reset() {
	*x = DataRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_agent_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DataRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DataRequest) ProtoMessage() {}

func (x *DataRequest) ProtoReflect() protoreflect.Message {
	mi := &file_agent_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DataRequest.ProtoReflect.Descriptor instead.
func (*DataRequest) Descriptor() ([]byte, []int) {
	return file_agent_proto_rawDescGZIP(), []int{3}
}

func (x *DataRequest) GetTimestamp() int64 {
	if x != nil {
		return x.Timestamp
	}
	return 0
}

func (x *DataRequest) GetData() string {
	if x != nil {
		return x.Data
	}
	return ""
}

var File_agent_proto protoreflect.FileDescriptor

var file_agent_proto_rawDesc = []byte{
	0x0a, 0x0b, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x05, 0x61,
	0x67, 0x65, 0x6e, 0x74, 0x22, 0x07, 0x0a, 0x05, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x22, 0x31, 0x0a,
	0x11, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x12, 0x1c, 0x0a, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70,
	0x22, 0x61, 0x0a, 0x12, 0x43, 0x72, 0x65, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x61, 0x6c, 0x73, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x17, 0x0a, 0x07, 0x70, 0x75, 0x62, 0x5f, 0x6b, 0x65,
	0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x70, 0x75, 0x62, 0x4b, 0x65, 0x79, 0x12,
	0x32, 0x0a, 0x0b, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x0e, 0x32, 0x11, 0x2e, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x43, 0x6c, 0x69,
	0x65, 0x6e, 0x74, 0x54, 0x79, 0x70, 0x65, 0x52, 0x0a, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x54,
	0x79, 0x70, 0x65, 0x22, 0x3f, 0x0a, 0x0b, 0x44, 0x61, 0x74, 0x61, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x12, 0x1c, 0x0a, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70,
	0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04,
	0x64, 0x61, 0x74, 0x61, 0x2a, 0x45, 0x0a, 0x0a, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x54, 0x79,
	0x70, 0x65, 0x12, 0x0c, 0x0a, 0x08, 0x4c, 0x4e, 0x44, 0x5f, 0x47, 0x52, 0x50, 0x43, 0x10, 0x00,
	0x12, 0x0c, 0x0a, 0x08, 0x4c, 0x4e, 0x44, 0x5f, 0x52, 0x45, 0x53, 0x54, 0x10, 0x01, 0x12, 0x0f,
	0x0a, 0x0b, 0x43, 0x5f, 0x4c, 0x49, 0x47, 0x48, 0x54, 0x4e, 0x49, 0x4e, 0x47, 0x10, 0x02, 0x12,
	0x0a, 0x0a, 0x06, 0x45, 0x43, 0x4c, 0x41, 0x49, 0x52, 0x10, 0x03, 0x32, 0xe2, 0x03, 0x0a, 0x08,
	0x41, 0x67, 0x65, 0x6e, 0x74, 0x41, 0x50, 0x49, 0x12, 0x2e, 0x0a, 0x08, 0x50, 0x61, 0x79, 0x6d,
	0x65, 0x6e, 0x74, 0x73, 0x12, 0x12, 0x2e, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x44, 0x61, 0x74,
	0x61, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x0c, 0x2e, 0x61, 0x67, 0x65, 0x6e, 0x74,
	0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x22, 0x00, 0x12, 0x42, 0x0a, 0x16, 0x4c, 0x61, 0x74, 0x65,
	0x73, 0x74, 0x50, 0x61, 0x79, 0x6d, 0x65, 0x6e, 0x74, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61,
	0x6d, 0x70, 0x12, 0x0c, 0x2e, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79,
	0x1a, 0x18, 0x2e, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61,
	0x6d, 0x70, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x2e, 0x0a, 0x08,
	0x49, 0x6e, 0x76, 0x6f, 0x69, 0x63, 0x65, 0x73, 0x12, 0x12, 0x2e, 0x61, 0x67, 0x65, 0x6e, 0x74,
	0x2e, 0x44, 0x61, 0x74, 0x61, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x0c, 0x2e, 0x61,
	0x67, 0x65, 0x6e, 0x74, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x22, 0x00, 0x12, 0x42, 0x0a, 0x16,
	0x4c, 0x61, 0x74, 0x65, 0x73, 0x74, 0x49, 0x6e, 0x76, 0x6f, 0x69, 0x63, 0x65, 0x54, 0x69, 0x6d,
	0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x12, 0x0c, 0x2e, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x45,
	0x6d, 0x70, 0x74, 0x79, 0x1a, 0x18, 0x2e, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x54, 0x69, 0x6d,
	0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00,
	0x12, 0x2e, 0x0a, 0x08, 0x46, 0x6f, 0x72, 0x77, 0x61, 0x72, 0x64, 0x73, 0x12, 0x12, 0x2e, 0x61,
	0x67, 0x65, 0x6e, 0x74, 0x2e, 0x44, 0x61, 0x74, 0x61, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x1a, 0x0c, 0x2e, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x22, 0x00,
	0x12, 0x42, 0x0a, 0x16, 0x4c, 0x61, 0x74, 0x65, 0x73, 0x74, 0x46, 0x6f, 0x72, 0x77, 0x61, 0x72,
	0x64, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x12, 0x0c, 0x2e, 0x61, 0x67, 0x65,
	0x6e, 0x74, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x1a, 0x18, 0x2e, 0x61, 0x67, 0x65, 0x6e, 0x74,
	0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x22, 0x00, 0x12, 0x32, 0x0a, 0x0c, 0x4c, 0x69, 0x71, 0x75, 0x69, 0x64, 0x69, 0x74,
	0x79, 0x41, 0x64, 0x73, 0x12, 0x12, 0x2e, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x44, 0x61, 0x74,
	0x61, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x0c, 0x2e, 0x61, 0x67, 0x65, 0x6e, 0x74,
	0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x22, 0x00, 0x12, 0x46, 0x0a, 0x1a, 0x4c, 0x61, 0x74, 0x65,
	0x73, 0x74, 0x4c, 0x69, 0x71, 0x75, 0x69, 0x64, 0x69, 0x74, 0x79, 0x41, 0x64, 0x54, 0x69, 0x6d,
	0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x12, 0x0c, 0x2e, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x45,
	0x6d, 0x70, 0x74, 0x79, 0x1a, 0x18, 0x2e, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x54, 0x69, 0x6d,
	0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00,
	0x42, 0x32, 0x5a, 0x30, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x62,
	0x6f, 0x6c, 0x74, 0x2d, 0x6f, 0x62, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x2f, 0x62, 0x6f, 0x6c,
	0x74, 0x2d, 0x6f, 0x62, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x61,
	0x67, 0x65, 0x6e, 0x74, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_agent_proto_rawDescOnce sync.Once
	file_agent_proto_rawDescData = file_agent_proto_rawDesc
)

func file_agent_proto_rawDescGZIP() []byte {
	file_agent_proto_rawDescOnce.Do(func() {
		file_agent_proto_rawDescData = protoimpl.X.CompressGZIP(file_agent_proto_rawDescData)
	})
	return file_agent_proto_rawDescData
}

var file_agent_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_agent_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_agent_proto_goTypes = []interface{}{
	(ClientType)(0),            // 0: agent.ClientType
	(*Empty)(nil),              // 1: agent.Empty
	(*TimestampResponse)(nil),  // 2: agent.TimestampResponse
	(*CredentialsRequest)(nil), // 3: agent.CredentialsRequest
	(*DataRequest)(nil),        // 4: agent.DataRequest
}
var file_agent_proto_depIdxs = []int32{
	0, // 0: agent.CredentialsRequest.client_type:type_name -> agent.ClientType
	4, // 1: agent.AgentAPI.Payments:input_type -> agent.DataRequest
	1, // 2: agent.AgentAPI.LatestPaymentTimestamp:input_type -> agent.Empty
	4, // 3: agent.AgentAPI.Invoices:input_type -> agent.DataRequest
	1, // 4: agent.AgentAPI.LatestInvoiceTimestamp:input_type -> agent.Empty
	4, // 5: agent.AgentAPI.Forwards:input_type -> agent.DataRequest
	1, // 6: agent.AgentAPI.LatestForwardTimestamp:input_type -> agent.Empty
	4, // 7: agent.AgentAPI.LiquidityAds:input_type -> agent.DataRequest
	1, // 8: agent.AgentAPI.LatestLiquidityAdTimestamp:input_type -> agent.Empty
	1, // 9: agent.AgentAPI.Payments:output_type -> agent.Empty
	2, // 10: agent.AgentAPI.LatestPaymentTimestamp:output_type -> agent.TimestampResponse
	1, // 11: agent.AgentAPI.Invoices:output_type -> agent.Empty
	2, // 12: agent.AgentAPI.LatestInvoiceTimestamp:output_type -> agent.TimestampResponse
	1, // 13: agent.AgentAPI.Forwards:output_type -> agent.Empty
	2, // 14: agent.AgentAPI.LatestForwardTimestamp:output_type -> agent.TimestampResponse
	1, // 15: agent.AgentAPI.LiquidityAds:output_type -> agent.Empty
	2, // 16: agent.AgentAPI.LatestLiquidityAdTimestamp:output_type -> agent.TimestampResponse
	9, // [9:17] is the sub-list for method output_type
	1, // [1:9] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_agent_proto_init() }
func file_agent_proto_init() {
	if File_agent_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_agent_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Empty); i {
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
		file_agent_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TimestampResponse); i {
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
		file_agent_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CredentialsRequest); i {
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
		file_agent_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DataRequest); i {
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
			RawDescriptor: file_agent_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_agent_proto_goTypes,
		DependencyIndexes: file_agent_proto_depIdxs,
		EnumInfos:         file_agent_proto_enumTypes,
		MessageInfos:      file_agent_proto_msgTypes,
	}.Build()
	File_agent_proto = out.File
	file_agent_proto_rawDesc = nil
	file_agent_proto_goTypes = nil
	file_agent_proto_depIdxs = nil
}
