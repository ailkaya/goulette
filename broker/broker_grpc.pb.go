// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.29.0--rc1
// source: broker.proto

package broker

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	BrokerService_RegisterTopic_FullMethodName   = "/broker.BrokerService/RegisterTopic"
	BrokerService_SendMessage_FullMethodName     = "/broker.BrokerService/SendMessage"
	BrokerService_RetrieveMessage_FullMethodName = "/broker.BrokerService/RetrieveMessage"
)

// BrokerServiceClient is the client API for BrokerService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type BrokerServiceClient interface {
	RegisterTopic(ctx context.Context, in *TopicRequest, opts ...grpc.CallOption) (*Empty, error)
	SendMessage(ctx context.Context, opts ...grpc.CallOption) (grpc.ClientStreamingClient[SendMessageRequest, Empty], error)
	RetrieveMessage(ctx context.Context, in *TopicRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[RetrieveMessageResponse], error)
}

type brokerServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewBrokerServiceClient(cc grpc.ClientConnInterface) BrokerServiceClient {
	return &brokerServiceClient{cc}
}

func (c *brokerServiceClient) RegisterTopic(ctx context.Context, in *TopicRequest, opts ...grpc.CallOption) (*Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Empty)
	err := c.cc.Invoke(ctx, BrokerService_RegisterTopic_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *brokerServiceClient) SendMessage(ctx context.Context, opts ...grpc.CallOption) (grpc.ClientStreamingClient[SendMessageRequest, Empty], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &BrokerService_ServiceDesc.Streams[0], BrokerService_SendMessage_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[SendMessageRequest, Empty]{ClientStream: stream}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type BrokerService_SendMessageClient = grpc.ClientStreamingClient[SendMessageRequest, Empty]

func (c *brokerServiceClient) RetrieveMessage(ctx context.Context, in *TopicRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[RetrieveMessageResponse], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &BrokerService_ServiceDesc.Streams[1], BrokerService_RetrieveMessage_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[TopicRequest, RetrieveMessageResponse]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type BrokerService_RetrieveMessageClient = grpc.ServerStreamingClient[RetrieveMessageResponse]

// BrokerServiceServer is the server API for BrokerService service.
// All implementations must embed UnimplementedBrokerServiceServer
// for forward compatibility.
type BrokerServiceServer interface {
	RegisterTopic(context.Context, *TopicRequest) (*Empty, error)
	SendMessage(grpc.ClientStreamingServer[SendMessageRequest, Empty]) error
	RetrieveMessage(*TopicRequest, grpc.ServerStreamingServer[RetrieveMessageResponse]) error
	mustEmbedUnimplementedBrokerServiceServer()
}

// UnimplementedBrokerServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedBrokerServiceServer struct{}

func (UnimplementedBrokerServiceServer) RegisterTopic(context.Context, *TopicRequest) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RegisterTopic not implemented")
}
func (UnimplementedBrokerServiceServer) SendMessage(grpc.ClientStreamingServer[SendMessageRequest, Empty]) error {
	return status.Errorf(codes.Unimplemented, "method SendMessage not implemented")
}
func (UnimplementedBrokerServiceServer) RetrieveMessage(*TopicRequest, grpc.ServerStreamingServer[RetrieveMessageResponse]) error {
	return status.Errorf(codes.Unimplemented, "method RetrieveMessage not implemented")
}
func (UnimplementedBrokerServiceServer) mustEmbedUnimplementedBrokerServiceServer() {}
func (UnimplementedBrokerServiceServer) testEmbeddedByValue()                       {}

// UnsafeBrokerServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to BrokerServiceServer will
// result in compilation errors.
type UnsafeBrokerServiceServer interface {
	mustEmbedUnimplementedBrokerServiceServer()
}

func RegisterBrokerServiceServer(s grpc.ServiceRegistrar, srv BrokerServiceServer) {
	// If the following call pancis, it indicates UnimplementedBrokerServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&BrokerService_ServiceDesc, srv)
}

func _BrokerService_RegisterTopic_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TopicRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BrokerServiceServer).RegisterTopic(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BrokerService_RegisterTopic_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BrokerServiceServer).RegisterTopic(ctx, req.(*TopicRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _BrokerService_SendMessage_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(BrokerServiceServer).SendMessage(&grpc.GenericServerStream[SendMessageRequest, Empty]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type BrokerService_SendMessageServer = grpc.ClientStreamingServer[SendMessageRequest, Empty]

func _BrokerService_RetrieveMessage_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(TopicRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(BrokerServiceServer).RetrieveMessage(m, &grpc.GenericServerStream[TopicRequest, RetrieveMessageResponse]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type BrokerService_RetrieveMessageServer = grpc.ServerStreamingServer[RetrieveMessageResponse]

// BrokerService_ServiceDesc is the grpc.ServiceDesc for BrokerService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var BrokerService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "broker.BrokerService",
	HandlerType: (*BrokerServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "RegisterTopic",
			Handler:    _BrokerService_RegisterTopic_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "SendMessage",
			Handler:       _BrokerService_SendMessage_Handler,
			ClientStreams: true,
		},
		{
			StreamName:    "RetrieveMessage",
			Handler:       _BrokerService_RetrieveMessage_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "broker.proto",
}
