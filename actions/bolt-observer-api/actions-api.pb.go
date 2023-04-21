// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.29.1
// 	protoc        v3.21.12
// source: actions-api.proto

package bolt_observer_api

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

type ReplyType int32

const (
	ReplyType_SUCCESS ReplyType = 0
	ReplyType_ERROR   ReplyType = 1
)

// Enum value maps for ReplyType.
var (
	ReplyType_name = map[int32]string{
		0: "SUCCESS",
		1: "ERROR",
	}
	ReplyType_value = map[string]int32{
		"SUCCESS": 0,
		"ERROR":   1,
	}
)

func (x ReplyType) Enum() *ReplyType {
	p := new(ReplyType)
	*p = x
	return p
}

func (x ReplyType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ReplyType) Descriptor() protoreflect.EnumDescriptor {
	return file_actions_api_proto_enumTypes[0].Descriptor()
}

func (ReplyType) Type() protoreflect.EnumType {
	return &file_actions_api_proto_enumTypes[0]
}

func (x ReplyType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ReplyType.Descriptor instead.
func (ReplyType) EnumDescriptor() ([]byte, []int) {
	return file_actions_api_proto_rawDescGZIP(), []int{0}
}

type Sequence int32

const (
	Sequence_CONNECT  Sequence = 0
	Sequence_EXECUTE  Sequence = 1
	Sequence_LOG      Sequence = 2
	Sequence_FINISHED Sequence = 3
)

// Enum value maps for Sequence.
var (
	Sequence_name = map[int32]string{
		0: "CONNECT",
		1: "EXECUTE",
		2: "LOG",
		3: "FINISHED",
	}
	Sequence_value = map[string]int32{
		"CONNECT":  0,
		"EXECUTE":  1,
		"LOG":      2,
		"FINISHED": 3,
	}
)

func (x Sequence) Enum() *Sequence {
	p := new(Sequence)
	*p = x
	return p
}

func (x Sequence) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Sequence) Descriptor() protoreflect.EnumDescriptor {
	return file_actions_api_proto_enumTypes[1].Descriptor()
}

func (Sequence) Type() protoreflect.EnumType {
	return &file_actions_api_proto_enumTypes[1]
}

func (x Sequence) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Sequence.Descriptor instead.
func (Sequence) EnumDescriptor() ([]byte, []int) {
	return file_actions_api_proto_rawDescGZIP(), []int{1}
}

type Action struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	JobId    int64    `protobuf:"varint,1,opt,name=jobId,proto3" json:"jobId,omitempty"`
	Sequence Sequence `protobuf:"varint,2,opt,name=sequence,proto3,enum=agent.Sequence" json:"sequence,omitempty"`
	Action   string   `protobuf:"bytes,3,opt,name=action,proto3" json:"action,omitempty"`
	Data     []byte   `protobuf:"bytes,4,opt,name=data,proto3" json:"data,omitempty"`
}

func (x *Action) Reset() {
	*x = Action{}
	if protoimpl.UnsafeEnabled {
		mi := &file_actions_api_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Action) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Action) ProtoMessage() {}

func (x *Action) ProtoReflect() protoreflect.Message {
	mi := &file_actions_api_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Action.ProtoReflect.Descriptor instead.
func (*Action) Descriptor() ([]byte, []int) {
	return file_actions_api_proto_rawDescGZIP(), []int{0}
}

func (x *Action) GetJobId() int64 {
	if x != nil {
		return x.JobId
	}
	return 0
}

func (x *Action) GetSequence() Sequence {
	if x != nil {
		return x.Sequence
	}
	return Sequence_CONNECT
}

func (x *Action) GetAction() string {
	if x != nil {
		return x.Action
	}
	return ""
}

func (x *Action) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

type AgentReply struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	JobId    int64     `protobuf:"varint,1,opt,name=jobId,proto3" json:"jobId,omitempty"`
	Sequence Sequence  `protobuf:"varint,2,opt,name=sequence,proto3,enum=agent.Sequence" json:"sequence,omitempty"`
	Type     ReplyType `protobuf:"varint,3,opt,name=type,proto3,enum=agent.ReplyType" json:"type,omitempty"`
	Message  string    `protobuf:"bytes,4,opt,name=message,proto3" json:"message,omitempty"`
	Data     []byte    `protobuf:"bytes,5,opt,name=data,proto3" json:"data,omitempty"`
}

func (x *AgentReply) Reset() {
	*x = AgentReply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_actions_api_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AgentReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AgentReply) ProtoMessage() {}

func (x *AgentReply) ProtoReflect() protoreflect.Message {
	mi := &file_actions_api_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AgentReply.ProtoReflect.Descriptor instead.
func (*AgentReply) Descriptor() ([]byte, []int) {
	return file_actions_api_proto_rawDescGZIP(), []int{1}
}

func (x *AgentReply) GetJobId() int64 {
	if x != nil {
		return x.JobId
	}
	return 0
}

func (x *AgentReply) GetSequence() Sequence {
	if x != nil {
		return x.Sequence
	}
	return Sequence_CONNECT
}

func (x *AgentReply) GetType() ReplyType {
	if x != nil {
		return x.Type
	}
	return ReplyType_SUCCESS
}

func (x *AgentReply) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

func (x *AgentReply) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

var File_actions_api_proto protoreflect.FileDescriptor

var file_actions_api_proto_rawDesc = []byte{
	0x0a, 0x11, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2d, 0x61, 0x70, 0x69, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x12, 0x05, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x22, 0x77, 0x0a, 0x06, 0x41, 0x63,
	0x74, 0x69, 0x6f, 0x6e, 0x12, 0x14, 0x0a, 0x05, 0x6a, 0x6f, 0x62, 0x49, 0x64, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x03, 0x52, 0x05, 0x6a, 0x6f, 0x62, 0x49, 0x64, 0x12, 0x2b, 0x0a, 0x08, 0x73, 0x65,
	0x71, 0x75, 0x65, 0x6e, 0x63, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x0f, 0x2e, 0x61,
	0x67, 0x65, 0x6e, 0x74, 0x2e, 0x53, 0x65, 0x71, 0x75, 0x65, 0x6e, 0x63, 0x65, 0x52, 0x08, 0x73,
	0x65, 0x71, 0x75, 0x65, 0x6e, 0x63, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x61, 0x63, 0x74, 0x69, 0x6f,
	0x6e, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x12,
	0x12, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x64,
	0x61, 0x74, 0x61, 0x22, 0xa3, 0x01, 0x0a, 0x0a, 0x41, 0x67, 0x65, 0x6e, 0x74, 0x52, 0x65, 0x70,
	0x6c, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x6a, 0x6f, 0x62, 0x49, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x03, 0x52, 0x05, 0x6a, 0x6f, 0x62, 0x49, 0x64, 0x12, 0x2b, 0x0a, 0x08, 0x73, 0x65, 0x71, 0x75,
	0x65, 0x6e, 0x63, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x0f, 0x2e, 0x61, 0x67, 0x65,
	0x6e, 0x74, 0x2e, 0x53, 0x65, 0x71, 0x75, 0x65, 0x6e, 0x63, 0x65, 0x52, 0x08, 0x73, 0x65, 0x71,
	0x75, 0x65, 0x6e, 0x63, 0x65, 0x12, 0x24, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x0e, 0x32, 0x10, 0x2e, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x52, 0x65, 0x70, 0x6c,
	0x79, 0x54, 0x79, 0x70, 0x65, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x6d,
	0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6d, 0x65,
	0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x05, 0x20,
	0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x2a, 0x23, 0x0a, 0x09, 0x52, 0x65, 0x70,
	0x6c, 0x79, 0x54, 0x79, 0x70, 0x65, 0x12, 0x0b, 0x0a, 0x07, 0x53, 0x55, 0x43, 0x43, 0x45, 0x53,
	0x53, 0x10, 0x00, 0x12, 0x09, 0x0a, 0x05, 0x45, 0x52, 0x52, 0x4f, 0x52, 0x10, 0x01, 0x2a, 0x3b,
	0x0a, 0x08, 0x53, 0x65, 0x71, 0x75, 0x65, 0x6e, 0x63, 0x65, 0x12, 0x0b, 0x0a, 0x07, 0x43, 0x4f,
	0x4e, 0x4e, 0x45, 0x43, 0x54, 0x10, 0x00, 0x12, 0x0b, 0x0a, 0x07, 0x45, 0x58, 0x45, 0x43, 0x55,
	0x54, 0x45, 0x10, 0x01, 0x12, 0x07, 0x0a, 0x03, 0x4c, 0x4f, 0x47, 0x10, 0x02, 0x12, 0x0c, 0x0a,
	0x08, 0x46, 0x49, 0x4e, 0x49, 0x53, 0x48, 0x45, 0x44, 0x10, 0x03, 0x32, 0x40, 0x0a, 0x09, 0x41,
	0x63, 0x74, 0x69, 0x6f, 0x6e, 0x41, 0x50, 0x49, 0x12, 0x33, 0x0a, 0x09, 0x53, 0x75, 0x62, 0x73,
	0x63, 0x72, 0x69, 0x62, 0x65, 0x12, 0x11, 0x2e, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x41, 0x67,
	0x65, 0x6e, 0x74, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x1a, 0x0d, 0x2e, 0x61, 0x67, 0x65, 0x6e, 0x74,
	0x2e, 0x41, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0x00, 0x28, 0x01, 0x30, 0x01, 0x42, 0x3a, 0x5a,
	0x38, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x62, 0x6f, 0x6c, 0x74,
	0x2d, 0x6f, 0x62, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x2f, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2f,
	0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2f, 0x62, 0x6f, 0x6c, 0x74, 0x2d, 0x6f, 0x62, 0x73,
	0x65, 0x72, 0x76, 0x65, 0x72, 0x2d, 0x61, 0x70, 0x69, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x33,
}

var (
	file_actions_api_proto_rawDescOnce sync.Once
	file_actions_api_proto_rawDescData = file_actions_api_proto_rawDesc
)

func file_actions_api_proto_rawDescGZIP() []byte {
	file_actions_api_proto_rawDescOnce.Do(func() {
		file_actions_api_proto_rawDescData = protoimpl.X.CompressGZIP(file_actions_api_proto_rawDescData)
	})
	return file_actions_api_proto_rawDescData
}

var file_actions_api_proto_enumTypes = make([]protoimpl.EnumInfo, 2)
var file_actions_api_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_actions_api_proto_goTypes = []interface{}{
	(ReplyType)(0),     // 0: agent.ReplyType
	(Sequence)(0),      // 1: agent.Sequence
	(*Action)(nil),     // 2: agent.Action
	(*AgentReply)(nil), // 3: agent.AgentReply
}
var file_actions_api_proto_depIdxs = []int32{
	1, // 0: agent.Action.sequence:type_name -> agent.Sequence
	1, // 1: agent.AgentReply.sequence:type_name -> agent.Sequence
	0, // 2: agent.AgentReply.type:type_name -> agent.ReplyType
	3, // 3: agent.ActionAPI.Subscribe:input_type -> agent.AgentReply
	2, // 4: agent.ActionAPI.Subscribe:output_type -> agent.Action
	4, // [4:5] is the sub-list for method output_type
	3, // [3:4] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_actions_api_proto_init() }
func file_actions_api_proto_init() {
	if File_actions_api_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_actions_api_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Action); i {
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
		file_actions_api_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AgentReply); i {
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
			RawDescriptor: file_actions_api_proto_rawDesc,
			NumEnums:      2,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_actions_api_proto_goTypes,
		DependencyIndexes: file_actions_api_proto_depIdxs,
		EnumInfos:         file_actions_api_proto_enumTypes,
		MessageInfos:      file_actions_api_proto_msgTypes,
	}.Build()
	File_actions_api_proto = out.File
	file_actions_api_proto_rawDesc = nil
	file_actions_api_proto_goTypes = nil
	file_actions_api_proto_depIdxs = nil
}
