package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc"
)

var _ grpc.ClientConnInterface = &ClientConn{}

type ClientConn struct{}

func (c *ClientConn) Invoke(ctx context.Context, method string, args any, reply any, opts ...grpc.CallOption) error {
	// TODO implement me
	panic("implement me")
}

func (c *ClientConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, errors.New("streams are not supported by Wasm")
}
