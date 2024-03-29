// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v3.21.12
// source: actions-api.proto

package bolt_observer_api

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
	ActionAPI_Subscribe_FullMethodName = "/agent.ActionAPI/Subscribe"
)

// ActionAPIClient is the client API for ActionAPI service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ActionAPIClient interface {
	Subscribe(ctx context.Context, opts ...grpc.CallOption) (ActionAPI_SubscribeClient, error)
}

type actionAPIClient struct {
	cc grpc.ClientConnInterface
}

func NewActionAPIClient(cc grpc.ClientConnInterface) ActionAPIClient {
	return &actionAPIClient{cc}
}

func (c *actionAPIClient) Subscribe(ctx context.Context, opts ...grpc.CallOption) (ActionAPI_SubscribeClient, error) {
	stream, err := c.cc.NewStream(ctx, &ActionAPI_ServiceDesc.Streams[0], ActionAPI_Subscribe_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &actionAPISubscribeClient{stream}
	return x, nil
}

type ActionAPI_SubscribeClient interface {
	Send(*AgentReply) error
	Recv() (*Action, error)
	grpc.ClientStream
}

type actionAPISubscribeClient struct {
	grpc.ClientStream
}

func (x *actionAPISubscribeClient) Send(m *AgentReply) error {
	return x.ClientStream.SendMsg(m)
}

func (x *actionAPISubscribeClient) Recv() (*Action, error) {
	m := new(Action)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// ActionAPIServer is the server API for ActionAPI service.
// All implementations must embed UnimplementedActionAPIServer
// for forward compatibility
type ActionAPIServer interface {
	Subscribe(ActionAPI_SubscribeServer) error
	mustEmbedUnimplementedActionAPIServer()
}

// UnimplementedActionAPIServer must be embedded to have forward compatible implementations.
type UnimplementedActionAPIServer struct {
}

func (UnimplementedActionAPIServer) Subscribe(ActionAPI_SubscribeServer) error {
	return status.Errorf(codes.Unimplemented, "method Subscribe not implemented")
}
func (UnimplementedActionAPIServer) mustEmbedUnimplementedActionAPIServer() {}

// UnsafeActionAPIServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ActionAPIServer will
// result in compilation errors.
type UnsafeActionAPIServer interface {
	mustEmbedUnimplementedActionAPIServer()
}

func RegisterActionAPIServer(s grpc.ServiceRegistrar, srv ActionAPIServer) {
	s.RegisterService(&ActionAPI_ServiceDesc, srv)
}

func _ActionAPI_Subscribe_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(ActionAPIServer).Subscribe(&actionAPISubscribeServer{stream})
}

type ActionAPI_SubscribeServer interface {
	Send(*Action) error
	Recv() (*AgentReply, error)
	grpc.ServerStream
}

type actionAPISubscribeServer struct {
	grpc.ServerStream
}

func (x *actionAPISubscribeServer) Send(m *Action) error {
	return x.ServerStream.SendMsg(m)
}

func (x *actionAPISubscribeServer) Recv() (*AgentReply, error) {
	m := new(AgentReply)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// ActionAPI_ServiceDesc is the grpc.ServiceDesc for ActionAPI service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var ActionAPI_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "agent.ActionAPI",
	HandlerType: (*ActionAPIServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Subscribe",
			Handler:       _ActionAPI_Subscribe_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "actions-api.proto",
}
