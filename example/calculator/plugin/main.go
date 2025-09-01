package main

import (
	"context"

	"github.com/lovromazgon/hornet/example/calculator/sdk"
	calculatorv1 "github.com/lovromazgon/hornet/example/calculator/sdk/proto/calculator/v1"
	"github.com/lovromazgon/hornet/grpc"
	"github.com/lovromazgon/hornet/wasm"
)

// -- Plugin setup -------------------------------------------------------------

func main() {
	// The main function is required by the compiler, but will never actually
	// be called, so it can stay empty.
}

func init() {
	// The plugin is initialized in init.
	srv := grpc.NewServer()
	calculatorv1.RegisterCalculatorServer(srv, &CalculatorServer{impl: &Calculator{}})
	wasm.Init(srv)
}

// -- Plugin gRPC server -------------------------------------------------------

var _ calculatorv1.CalculatorServer = (*CalculatorServer)(nil)

type CalculatorServer struct {
	calculatorv1.UnimplementedCalculatorServer
	impl sdk.Calculator
}

func (c *CalculatorServer) Add(ctx context.Context, req *calculatorv1.Add_Request) (*calculatorv1.Add_Response, error) {
	out := c.impl.Add(req.A, req.B)
	return &calculatorv1.Add_Response{C: out}, nil
}

func (c *CalculatorServer) Sub(ctx context.Context, req *calculatorv1.Sub_Request) (*calculatorv1.Sub_Response, error) {
	out := c.impl.Sub(req.A, req.B)
	return &calculatorv1.Sub_Response{C: out}, nil
}

func (c *CalculatorServer) Mul(ctx context.Context, req *calculatorv1.Mul_Request) (*calculatorv1.Mul_Response, error) {
	out := c.impl.Mul(req.A, req.B)
	return &calculatorv1.Mul_Response{C: out}, nil
}

func (c *CalculatorServer) Div(ctx context.Context, req *calculatorv1.Div_Request) (*calculatorv1.Div_Response, error) {
	out := c.impl.Div(req.A, req.B)
	return &calculatorv1.Div_Response{C: out}, nil
}

// -- Plugin implementation ----------------------------------------------------

// Calculator implements the interface defined in the SDK.
var _ sdk.Calculator = (*Calculator)(nil)

type Calculator struct{}

func (c *Calculator) Add(a, b int64) int64 {
	return a + b
}

func (c *Calculator) Sub(a, b int64) int64 {
	return a - b
}

func (c *Calculator) Mul(a, b int64) int64 {
	return a * b
}

func (c *Calculator) Div(a, b int64) int64 {
	return a / b
}
