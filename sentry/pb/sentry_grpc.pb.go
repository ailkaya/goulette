// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.29.0--rc1
// source: sentry.proto

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
	SentryService_RegisterTopic_FullMethodName         = "/pb.SentryService/RegisterTopic"
	SentryService_RegisterBroker_FullMethodName        = "/pb.SentryService/RegisterBroker"
	SentryService_GetBrokersForProducer_FullMethodName = "/pb.SentryService/GetBrokersForProducer"
	SentryService_GetBrokersForConsumer_FullMethodName = "/pb.SentryService/GetBrokersForConsumer"
	SentryService_LogOff_FullMethodName                = "/pb.SentryService/LogOff"
	SentryService_NoSuchTopic_FullMethodName           = "/pb.SentryService/NoSuchTopic"
)

// SentryServiceClient is the client API for SentryService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type SentryServiceClient interface {
	RegisterTopic(ctx context.Context, in *OnlyTopic, opts ...grpc.CallOption) (*Empty, error)
	RegisterBroker(ctx context.Context, in *RegisterBrokerParams, opts ...grpc.CallOption) (*Empty, error)
	GetBrokersForProducer(ctx context.Context, in *OnlyTopic, opts ...grpc.CallOption) (*QueryResp, error)
	GetBrokersForConsumer(ctx context.Context, in *AddrAndTopic, opts ...grpc.CallOption) (*QueryResp, error)
	LogOff(ctx context.Context, in *OnlyAddr, opts ...grpc.CallOption) (*Empty, error)
	NoSuchTopic(ctx context.Context, in *NoSuchTopicParams, opts ...grpc.CallOption) (*Empty, error)
}

type sentryServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewSentryServiceClient(cc grpc.ClientConnInterface) SentryServiceClient {
	return &sentryServiceClient{cc}
}

func (c *sentryServiceClient) RegisterTopic(ctx context.Context, in *OnlyTopic, opts ...grpc.CallOption) (*Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Empty)
	err := c.cc.Invoke(ctx, SentryService_RegisterTopic_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *sentryServiceClient) RegisterBroker(ctx context.Context, in *RegisterBrokerParams, opts ...grpc.CallOption) (*Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Empty)
	err := c.cc.Invoke(ctx, SentryService_RegisterBroker_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *sentryServiceClient) GetBrokersForProducer(ctx context.Context, in *OnlyTopic, opts ...grpc.CallOption) (*QueryResp, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(QueryResp)
	err := c.cc.Invoke(ctx, SentryService_GetBrokersForProducer_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *sentryServiceClient) GetBrokersForConsumer(ctx context.Context, in *AddrAndTopic, opts ...grpc.CallOption) (*QueryResp, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(QueryResp)
	err := c.cc.Invoke(ctx, SentryService_GetBrokersForConsumer_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *sentryServiceClient) LogOff(ctx context.Context, in *OnlyAddr, opts ...grpc.CallOption) (*Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Empty)
	err := c.cc.Invoke(ctx, SentryService_LogOff_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *sentryServiceClient) NoSuchTopic(ctx context.Context, in *NoSuchTopicParams, opts ...grpc.CallOption) (*Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Empty)
	err := c.cc.Invoke(ctx, SentryService_NoSuchTopic_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SentryServiceServer is the server API for SentryService service.
// All implementations must embed UnimplementedSentryServiceServer
// for forward compatibility.
type SentryServiceServer interface {
	RegisterTopic(context.Context, *OnlyTopic) (*Empty, error)
	RegisterBroker(context.Context, *RegisterBrokerParams) (*Empty, error)
	GetBrokersForProducer(context.Context, *OnlyTopic) (*QueryResp, error)
	GetBrokersForConsumer(context.Context, *AddrAndTopic) (*QueryResp, error)
	LogOff(context.Context, *OnlyAddr) (*Empty, error)
	NoSuchTopic(context.Context, *NoSuchTopicParams) (*Empty, error)
	mustEmbedUnimplementedSentryServiceServer()
}

// UnimplementedSentryServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedSentryServiceServer struct{}

func (UnimplementedSentryServiceServer) RegisterTopic(context.Context, *OnlyTopic) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RegisterTopic not implemented")
}
func (UnimplementedSentryServiceServer) RegisterBroker(context.Context, *RegisterBrokerParams) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RegisterBroker not implemented")
}
func (UnimplementedSentryServiceServer) GetBrokersForProducer(context.Context, *OnlyTopic) (*QueryResp, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetBrokersForProducer not implemented")
}
func (UnimplementedSentryServiceServer) GetBrokersForConsumer(context.Context, *AddrAndTopic) (*QueryResp, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetBrokersForConsumer not implemented")
}
func (UnimplementedSentryServiceServer) LogOff(context.Context, *OnlyAddr) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method LogOff not implemented")
}
func (UnimplementedSentryServiceServer) NoSuchTopic(context.Context, *NoSuchTopicParams) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method NoSuchTopic not implemented")
}
func (UnimplementedSentryServiceServer) mustEmbedUnimplementedSentryServiceServer() {}
func (UnimplementedSentryServiceServer) testEmbeddedByValue()                       {}

// UnsafeSentryServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to SentryServiceServer will
// result in compilation errors.
type UnsafeSentryServiceServer interface {
	mustEmbedUnimplementedSentryServiceServer()
}

func RegisterSentryServiceServer(s grpc.ServiceRegistrar, srv SentryServiceServer) {
	// If the following call pancis, it indicates UnimplementedSentryServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&SentryService_ServiceDesc, srv)
}

func _SentryService_RegisterTopic_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(OnlyTopic)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SentryServiceServer).RegisterTopic(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SentryService_RegisterTopic_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SentryServiceServer).RegisterTopic(ctx, req.(*OnlyTopic))
	}
	return interceptor(ctx, in, info, handler)
}

func _SentryService_RegisterBroker_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RegisterBrokerParams)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SentryServiceServer).RegisterBroker(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SentryService_RegisterBroker_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SentryServiceServer).RegisterBroker(ctx, req.(*RegisterBrokerParams))
	}
	return interceptor(ctx, in, info, handler)
}

func _SentryService_GetBrokersForProducer_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(OnlyTopic)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SentryServiceServer).GetBrokersForProducer(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SentryService_GetBrokersForProducer_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SentryServiceServer).GetBrokersForProducer(ctx, req.(*OnlyTopic))
	}
	return interceptor(ctx, in, info, handler)
}

func _SentryService_GetBrokersForConsumer_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddrAndTopic)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SentryServiceServer).GetBrokersForConsumer(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SentryService_GetBrokersForConsumer_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SentryServiceServer).GetBrokersForConsumer(ctx, req.(*AddrAndTopic))
	}
	return interceptor(ctx, in, info, handler)
}

func _SentryService_LogOff_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(OnlyAddr)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SentryServiceServer).LogOff(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SentryService_LogOff_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SentryServiceServer).LogOff(ctx, req.(*OnlyAddr))
	}
	return interceptor(ctx, in, info, handler)
}

func _SentryService_NoSuchTopic_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(NoSuchTopicParams)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SentryServiceServer).NoSuchTopic(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SentryService_NoSuchTopic_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SentryServiceServer).NoSuchTopic(ctx, req.(*NoSuchTopicParams))
	}
	return interceptor(ctx, in, info, handler)
}

// SentryService_ServiceDesc is the grpc.ServiceDesc for SentryService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var SentryService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "pb.SentryService",
	HandlerType: (*SentryServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "RegisterTopic",
			Handler:    _SentryService_RegisterTopic_Handler,
		},
		{
			MethodName: "RegisterBroker",
			Handler:    _SentryService_RegisterBroker_Handler,
		},
		{
			MethodName: "GetBrokersForProducer",
			Handler:    _SentryService_GetBrokersForProducer_Handler,
		},
		{
			MethodName: "GetBrokersForConsumer",
			Handler:    _SentryService_GetBrokersForConsumer_Handler,
		},
		{
			MethodName: "LogOff",
			Handler:    _SentryService_LogOff_Handler,
		},
		{
			MethodName: "NoSuchTopic",
			Handler:    _SentryService_NoSuchTopic_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "sentry.proto",
}
