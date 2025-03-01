// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.29.0--rc1
// source: broker.proto

package pb

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
	BrokerService_Produce_FullMethodName = "/pb.BrokerService/Produce"
	BrokerService_Consume_FullMethodName = "/pb.BrokerService/Consume"
)

// BrokerServiceClient is the client API for BrokerService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type BrokerServiceClient interface {
	Produce(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[ProduceRequest, ProduceResponse], error)
	Consume(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[ConsumeRequest, ConsumeResponse], error)
}

type brokerServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewBrokerServiceClient(cc grpc.ClientConnInterface) BrokerServiceClient {
	return &brokerServiceClient{cc}
}

func (c *brokerServiceClient) Produce(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[ProduceRequest, ProduceResponse], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &BrokerService_ServiceDesc.Streams[0], BrokerService_Produce_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[ProduceRequest, ProduceResponse]{ClientStream: stream}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type BrokerService_ProduceClient = grpc.BidiStreamingClient[ProduceRequest, ProduceResponse]

func (c *brokerServiceClient) Consume(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[ConsumeRequest, ConsumeResponse], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &BrokerService_ServiceDesc.Streams[1], BrokerService_Consume_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[ConsumeRequest, ConsumeResponse]{ClientStream: stream}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type BrokerService_ConsumeClient = grpc.BidiStreamingClient[ConsumeRequest, ConsumeResponse]

// BrokerServiceServer is the server API for BrokerService service.
// All implementations must embed UnimplementedBrokerServiceServer
// for forward compatibility.
type BrokerServiceServer interface {
	Produce(grpc.BidiStreamingServer[ProduceRequest, ProduceResponse]) error
	Consume(grpc.BidiStreamingServer[ConsumeRequest, ConsumeResponse]) error
	mustEmbedUnimplementedBrokerServiceServer()
}

// UnimplementedBrokerServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedBrokerServiceServer struct{}

func (UnimplementedBrokerServiceServer) Produce(grpc.BidiStreamingServer[ProduceRequest, ProduceResponse]) error {
	return status.Errorf(codes.Unimplemented, "method Produce not implemented")
}
func (UnimplementedBrokerServiceServer) Consume(grpc.BidiStreamingServer[ConsumeRequest, ConsumeResponse]) error {
	return status.Errorf(codes.Unimplemented, "method Consume not implemented")
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

func _BrokerService_Produce_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(BrokerServiceServer).Produce(&grpc.GenericServerStream[ProduceRequest, ProduceResponse]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type BrokerService_ProduceServer = grpc.BidiStreamingServer[ProduceRequest, ProduceResponse]

func _BrokerService_Consume_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(BrokerServiceServer).Consume(&grpc.GenericServerStream[ConsumeRequest, ConsumeResponse]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type BrokerService_ConsumeServer = grpc.BidiStreamingServer[ConsumeRequest, ConsumeResponse]

// BrokerService_ServiceDesc is the grpc.ServiceDesc for BrokerService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var BrokerService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "pb.BrokerService",
	HandlerType: (*BrokerServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Produce",
			Handler:       _BrokerService_Produce_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
		{
			StreamName:    "Consume",
			Handler:       _BrokerService_Consume_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "broker.proto",
}
