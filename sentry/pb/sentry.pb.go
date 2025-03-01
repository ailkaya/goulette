// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.35.1
// 	protoc        v5.29.0--rc1
// source: sentry.proto

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

type Empty struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *Empty) Reset() {
	*x = Empty{}
	mi := &file_sentry_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Empty) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Empty) ProtoMessage() {}

func (x *Empty) ProtoReflect() protoreflect.Message {
	mi := &file_sentry_proto_msgTypes[0]
	if x != nil {
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
	return file_sentry_proto_rawDescGZIP(), []int{0}
}

type OnlyTopic struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Topic string `protobuf:"bytes,1,opt,name=topic,proto3" json:"topic,omitempty"`
}

func (x *OnlyTopic) Reset() {
	*x = OnlyTopic{}
	mi := &file_sentry_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *OnlyTopic) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*OnlyTopic) ProtoMessage() {}

func (x *OnlyTopic) ProtoReflect() protoreflect.Message {
	mi := &file_sentry_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use OnlyTopic.ProtoReflect.Descriptor instead.
func (*OnlyTopic) Descriptor() ([]byte, []int) {
	return file_sentry_proto_rawDescGZIP(), []int{1}
}

func (x *OnlyTopic) GetTopic() string {
	if x != nil {
		return x.Topic
	}
	return ""
}

type OnlyAddr struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Addr string `protobuf:"bytes,1,opt,name=addr,proto3" json:"addr,omitempty"`
}

func (x *OnlyAddr) Reset() {
	*x = OnlyAddr{}
	mi := &file_sentry_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *OnlyAddr) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*OnlyAddr) ProtoMessage() {}

func (x *OnlyAddr) ProtoReflect() protoreflect.Message {
	mi := &file_sentry_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use OnlyAddr.ProtoReflect.Descriptor instead.
func (*OnlyAddr) Descriptor() ([]byte, []int) {
	return file_sentry_proto_rawDescGZIP(), []int{2}
}

func (x *OnlyAddr) GetAddr() string {
	if x != nil {
		return x.Addr
	}
	return ""
}

type AddrAndTopic struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Addr  string `protobuf:"bytes,1,opt,name=addr,proto3" json:"addr,omitempty"`
	Topic string `protobuf:"bytes,2,opt,name=topic,proto3" json:"topic,omitempty"`
}

func (x *AddrAndTopic) Reset() {
	*x = AddrAndTopic{}
	mi := &file_sentry_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *AddrAndTopic) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AddrAndTopic) ProtoMessage() {}

func (x *AddrAndTopic) ProtoReflect() protoreflect.Message {
	mi := &file_sentry_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AddrAndTopic.ProtoReflect.Descriptor instead.
func (*AddrAndTopic) Descriptor() ([]byte, []int) {
	return file_sentry_proto_rawDescGZIP(), []int{3}
}

func (x *AddrAndTopic) GetAddr() string {
	if x != nil {
		return x.Addr
	}
	return ""
}

func (x *AddrAndTopic) GetTopic() string {
	if x != nil {
		return x.Topic
	}
	return ""
}

type QueryResp struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Brokers []string `protobuf:"bytes,1,rep,name=brokers,proto3" json:"brokers,omitempty"`
}

func (x *QueryResp) Reset() {
	*x = QueryResp{}
	mi := &file_sentry_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *QueryResp) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*QueryResp) ProtoMessage() {}

func (x *QueryResp) ProtoReflect() protoreflect.Message {
	mi := &file_sentry_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use QueryResp.ProtoReflect.Descriptor instead.
func (*QueryResp) Descriptor() ([]byte, []int) {
	return file_sentry_proto_rawDescGZIP(), []int{4}
}

func (x *QueryResp) GetBrokers() []string {
	if x != nil {
		return x.Brokers
	}
	return nil
}

type RegisterBrokerParams struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Addr         string  `protobuf:"bytes,1,opt,name=addr,proto3" json:"addr,omitempty"`
	WeightFactor float32 `protobuf:"fixed32,2,opt,name=weightFactor,proto3" json:"weightFactor,omitempty"`
}

func (x *RegisterBrokerParams) Reset() {
	*x = RegisterBrokerParams{}
	mi := &file_sentry_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RegisterBrokerParams) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RegisterBrokerParams) ProtoMessage() {}

func (x *RegisterBrokerParams) ProtoReflect() protoreflect.Message {
	mi := &file_sentry_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RegisterBrokerParams.ProtoReflect.Descriptor instead.
func (*RegisterBrokerParams) Descriptor() ([]byte, []int) {
	return file_sentry_proto_rawDescGZIP(), []int{5}
}

func (x *RegisterBrokerParams) GetAddr() string {
	if x != nil {
		return x.Addr
	}
	return ""
}

func (x *RegisterBrokerParams) GetWeightFactor() float32 {
	if x != nil {
		return x.WeightFactor
	}
	return 0
}

type NoSuchTopicParams struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	BrokerAddr string `protobuf:"bytes,1,opt,name=brokerAddr,proto3" json:"brokerAddr,omitempty"`
	Addr       string `protobuf:"bytes,2,opt,name=addr,proto3" json:"addr,omitempty"`
	Topic      string `protobuf:"bytes,3,opt,name=topic,proto3" json:"topic,omitempty"`
}

func (x *NoSuchTopicParams) Reset() {
	*x = NoSuchTopicParams{}
	mi := &file_sentry_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *NoSuchTopicParams) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NoSuchTopicParams) ProtoMessage() {}

func (x *NoSuchTopicParams) ProtoReflect() protoreflect.Message {
	mi := &file_sentry_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NoSuchTopicParams.ProtoReflect.Descriptor instead.
func (*NoSuchTopicParams) Descriptor() ([]byte, []int) {
	return file_sentry_proto_rawDescGZIP(), []int{6}
}

func (x *NoSuchTopicParams) GetBrokerAddr() string {
	if x != nil {
		return x.BrokerAddr
	}
	return ""
}

func (x *NoSuchTopicParams) GetAddr() string {
	if x != nil {
		return x.Addr
	}
	return ""
}

func (x *NoSuchTopicParams) GetTopic() string {
	if x != nil {
		return x.Topic
	}
	return ""
}

var File_sentry_proto protoreflect.FileDescriptor

var file_sentry_proto_rawDesc = []byte{
	0x0a, 0x0c, 0x73, 0x65, 0x6e, 0x74, 0x72, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x02,
	0x70, 0x62, 0x22, 0x07, 0x0a, 0x05, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x22, 0x21, 0x0a, 0x09, 0x4f,
	0x6e, 0x6c, 0x79, 0x54, 0x6f, 0x70, 0x69, 0x63, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x6f, 0x70, 0x69,
	0x63, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x74, 0x6f, 0x70, 0x69, 0x63, 0x22, 0x1e,
	0x0a, 0x08, 0x4f, 0x6e, 0x6c, 0x79, 0x41, 0x64, 0x64, 0x72, 0x12, 0x12, 0x0a, 0x04, 0x61, 0x64,
	0x64, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x61, 0x64, 0x64, 0x72, 0x22, 0x38,
	0x0a, 0x0c, 0x41, 0x64, 0x64, 0x72, 0x41, 0x6e, 0x64, 0x54, 0x6f, 0x70, 0x69, 0x63, 0x12, 0x12,
	0x0a, 0x04, 0x61, 0x64, 0x64, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x61, 0x64,
	0x64, 0x72, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x6f, 0x70, 0x69, 0x63, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x05, 0x74, 0x6f, 0x70, 0x69, 0x63, 0x22, 0x25, 0x0a, 0x09, 0x51, 0x75, 0x65, 0x72,
	0x79, 0x52, 0x65, 0x73, 0x70, 0x12, 0x18, 0x0a, 0x07, 0x62, 0x72, 0x6f, 0x6b, 0x65, 0x72, 0x73,
	0x18, 0x01, 0x20, 0x03, 0x28, 0x09, 0x52, 0x07, 0x62, 0x72, 0x6f, 0x6b, 0x65, 0x72, 0x73, 0x22,
	0x4e, 0x0a, 0x14, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x65, 0x72, 0x42, 0x72, 0x6f, 0x6b, 0x65,
	0x72, 0x50, 0x61, 0x72, 0x61, 0x6d, 0x73, 0x12, 0x12, 0x0a, 0x04, 0x61, 0x64, 0x64, 0x72, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x61, 0x64, 0x64, 0x72, 0x12, 0x22, 0x0a, 0x0c, 0x77,
	0x65, 0x69, 0x67, 0x68, 0x74, 0x46, 0x61, 0x63, 0x74, 0x6f, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x02, 0x52, 0x0c, 0x77, 0x65, 0x69, 0x67, 0x68, 0x74, 0x46, 0x61, 0x63, 0x74, 0x6f, 0x72, 0x22,
	0x5d, 0x0a, 0x11, 0x4e, 0x6f, 0x53, 0x75, 0x63, 0x68, 0x54, 0x6f, 0x70, 0x69, 0x63, 0x50, 0x61,
	0x72, 0x61, 0x6d, 0x73, 0x12, 0x1e, 0x0a, 0x0a, 0x62, 0x72, 0x6f, 0x6b, 0x65, 0x72, 0x41, 0x64,
	0x64, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x62, 0x72, 0x6f, 0x6b, 0x65, 0x72,
	0x41, 0x64, 0x64, 0x72, 0x12, 0x12, 0x0a, 0x04, 0x61, 0x64, 0x64, 0x72, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x04, 0x61, 0x64, 0x64, 0x72, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x6f, 0x70, 0x69,
	0x63, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x74, 0x6f, 0x70, 0x69, 0x63, 0x32, 0xb6,
	0x02, 0x0a, 0x0d, 0x53, 0x65, 0x6e, 0x74, 0x72, 0x79, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65,
	0x12, 0x29, 0x0a, 0x0d, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x65, 0x72, 0x54, 0x6f, 0x70, 0x69,
	0x63, 0x12, 0x0d, 0x2e, 0x70, 0x62, 0x2e, 0x4f, 0x6e, 0x6c, 0x79, 0x54, 0x6f, 0x70, 0x69, 0x63,
	0x1a, 0x09, 0x2e, 0x70, 0x62, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x12, 0x35, 0x0a, 0x0e, 0x52,
	0x65, 0x67, 0x69, 0x73, 0x74, 0x65, 0x72, 0x42, 0x72, 0x6f, 0x6b, 0x65, 0x72, 0x12, 0x18, 0x2e,
	0x70, 0x62, 0x2e, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x65, 0x72, 0x42, 0x72, 0x6f, 0x6b, 0x65,
	0x72, 0x50, 0x61, 0x72, 0x61, 0x6d, 0x73, 0x1a, 0x09, 0x2e, 0x70, 0x62, 0x2e, 0x45, 0x6d, 0x70,
	0x74, 0x79, 0x12, 0x35, 0x0a, 0x15, 0x47, 0x65, 0x74, 0x42, 0x72, 0x6f, 0x6b, 0x65, 0x72, 0x73,
	0x46, 0x6f, 0x72, 0x50, 0x72, 0x6f, 0x64, 0x75, 0x63, 0x65, 0x72, 0x12, 0x0d, 0x2e, 0x70, 0x62,
	0x2e, 0x4f, 0x6e, 0x6c, 0x79, 0x54, 0x6f, 0x70, 0x69, 0x63, 0x1a, 0x0d, 0x2e, 0x70, 0x62, 0x2e,
	0x51, 0x75, 0x65, 0x72, 0x79, 0x52, 0x65, 0x73, 0x70, 0x12, 0x38, 0x0a, 0x15, 0x47, 0x65, 0x74,
	0x42, 0x72, 0x6f, 0x6b, 0x65, 0x72, 0x73, 0x46, 0x6f, 0x72, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6d,
	0x65, 0x72, 0x12, 0x10, 0x2e, 0x70, 0x62, 0x2e, 0x41, 0x64, 0x64, 0x72, 0x41, 0x6e, 0x64, 0x54,
	0x6f, 0x70, 0x69, 0x63, 0x1a, 0x0d, 0x2e, 0x70, 0x62, 0x2e, 0x51, 0x75, 0x65, 0x72, 0x79, 0x52,
	0x65, 0x73, 0x70, 0x12, 0x21, 0x0a, 0x06, 0x4c, 0x6f, 0x67, 0x4f, 0x66, 0x66, 0x12, 0x0c, 0x2e,
	0x70, 0x62, 0x2e, 0x4f, 0x6e, 0x6c, 0x79, 0x41, 0x64, 0x64, 0x72, 0x1a, 0x09, 0x2e, 0x70, 0x62,
	0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x12, 0x2f, 0x0a, 0x0b, 0x4e, 0x6f, 0x53, 0x75, 0x63, 0x68,
	0x54, 0x6f, 0x70, 0x69, 0x63, 0x12, 0x15, 0x2e, 0x70, 0x62, 0x2e, 0x4e, 0x6f, 0x53, 0x75, 0x63,
	0x68, 0x54, 0x6f, 0x70, 0x69, 0x63, 0x50, 0x61, 0x72, 0x61, 0x6d, 0x73, 0x1a, 0x09, 0x2e, 0x70,
	0x62, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x42, 0x07, 0x5a, 0x05, 0x2e, 0x2e, 0x2f, 0x70, 0x62,
	0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_sentry_proto_rawDescOnce sync.Once
	file_sentry_proto_rawDescData = file_sentry_proto_rawDesc
)

func file_sentry_proto_rawDescGZIP() []byte {
	file_sentry_proto_rawDescOnce.Do(func() {
		file_sentry_proto_rawDescData = protoimpl.X.CompressGZIP(file_sentry_proto_rawDescData)
	})
	return file_sentry_proto_rawDescData
}

var file_sentry_proto_msgTypes = make([]protoimpl.MessageInfo, 7)
var file_sentry_proto_goTypes = []any{
	(*Empty)(nil),                // 0: pb.Empty
	(*OnlyTopic)(nil),            // 1: pb.OnlyTopic
	(*OnlyAddr)(nil),             // 2: pb.OnlyAddr
	(*AddrAndTopic)(nil),         // 3: pb.AddrAndTopic
	(*QueryResp)(nil),            // 4: pb.QueryResp
	(*RegisterBrokerParams)(nil), // 5: pb.RegisterBrokerParams
	(*NoSuchTopicParams)(nil),    // 6: pb.NoSuchTopicParams
}
var file_sentry_proto_depIdxs = []int32{
	1, // 0: pb.SentryService.RegisterTopic:input_type -> pb.OnlyTopic
	5, // 1: pb.SentryService.RegisterBroker:input_type -> pb.RegisterBrokerParams
	1, // 2: pb.SentryService.GetBrokersForProducer:input_type -> pb.OnlyTopic
	3, // 3: pb.SentryService.GetBrokersForConsumer:input_type -> pb.AddrAndTopic
	2, // 4: pb.SentryService.LogOff:input_type -> pb.OnlyAddr
	6, // 5: pb.SentryService.NoSuchTopic:input_type -> pb.NoSuchTopicParams
	0, // 6: pb.SentryService.RegisterTopic:output_type -> pb.Empty
	0, // 7: pb.SentryService.RegisterBroker:output_type -> pb.Empty
	4, // 8: pb.SentryService.GetBrokersForProducer:output_type -> pb.QueryResp
	4, // 9: pb.SentryService.GetBrokersForConsumer:output_type -> pb.QueryResp
	0, // 10: pb.SentryService.LogOff:output_type -> pb.Empty
	0, // 11: pb.SentryService.NoSuchTopic:output_type -> pb.Empty
	6, // [6:12] is the sub-list for method output_type
	0, // [0:6] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_sentry_proto_init() }
func file_sentry_proto_init() {
	if File_sentry_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_sentry_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   7,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_sentry_proto_goTypes,
		DependencyIndexes: file_sentry_proto_depIdxs,
		MessageInfos:      file_sentry_proto_msgTypes,
	}.Build()
	File_sentry_proto = out.File
	file_sentry_proto_rawDesc = nil
	file_sentry_proto_goTypes = nil
	file_sentry_proto_depIdxs = nil
}
