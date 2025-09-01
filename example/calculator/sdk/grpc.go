package sdk

import (
	"context"
	"fmt"

	calculatorv1 "github.com/lovromazgon/hornet/example/calculator/sdk/proto/calculator/v1"
	hgrpc "github.com/lovromazgon/hornet/grpc"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"google.golang.org/grpc"
)

func InitializeModuleAndCalculator(
	ctx context.Context,
	runtime wazero.Runtime,
	source []byte,
) (api.Module, Calculator, error) {
	module, calc, err := hgrpc.InstantiateModuleAndClient(ctx, runtime, source, calculatorv1.NewCalculatorClient)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate Wasm module: %w", err))
	}
	return module, NewCalculatorFromClient(calc), nil
}

func NewCalculatorFromClient(client calculatorv1.CalculatorClient) Calculator {
	return &calculatorClient{client: client}
}

type calculatorClient struct {
	client calculatorv1.CalculatorClient
}

var _ Calculator = (*calculatorClient)(nil)

func (c *calculatorClient) Add(ctx context.Context, a, b int64) (int64, error) {
	out, err := c.client.Add(ctx, &calculatorv1.Add_Request{A: a, B: b})
	if err != nil {
		return 0, err
	}
	return out.C, nil
}

func (c *calculatorClient) Sub(ctx context.Context, a, b int64) (int64, error) {
	out, err := c.client.Sub(ctx, &calculatorv1.Sub_Request{A: a, B: b})
	if err != nil {
		return 0, err
	}
	return out.C, nil
}

func (c *calculatorClient) Mul(ctx context.Context, a, b int64) (int64, error) {
	out, err := c.client.Mul(ctx, &calculatorv1.Mul_Request{A: a, B: b})
	if err != nil {
		return 0, err
	}
	return out.C, nil
}

func (c *calculatorClient) Div(ctx context.Context, a, b int64) (int64, error) {
	out, err := c.client.Div(ctx, &calculatorv1.Div_Request{A: a, B: b})
	if err != nil {
		return 0, err
	}
	return out.C, nil
}

// RegisterCalculatorServer registers the Calculator implementation on the grpc
// service registrar (grpc.Server). Use this method when initializing the plugin.
func RegisterCalculatorServer(srv grpc.ServiceRegistrar, calc Calculator) {
	calculatorv1.RegisterCalculatorServer(srv, &calculatorServer{impl: calc})
}

// calculatorServer is a utility struct, an adapter that wraps a Calculator and
// exposes it as a server that implements the proto server definition.
type calculatorServer struct {
	calculatorv1.UnimplementedCalculatorServer
	impl Calculator
}

var _ calculatorv1.CalculatorServer = (*calculatorServer)(nil)

func (c *calculatorServer) Add(ctx context.Context, req *calculatorv1.Add_Request) (*calculatorv1.Add_Response, error) {
	out, err := c.impl.Add(ctx, req.A, req.B)
	if err != nil {
		return nil, err
	}
	return &calculatorv1.Add_Response{C: out}, nil
}

func (c *calculatorServer) Sub(ctx context.Context, req *calculatorv1.Sub_Request) (*calculatorv1.Sub_Response, error) {
	out, err := c.impl.Sub(ctx, req.A, req.B)
	if err != nil {
		return nil, err
	}
	return &calculatorv1.Sub_Response{C: out}, nil
}

func (c *calculatorServer) Mul(ctx context.Context, req *calculatorv1.Mul_Request) (*calculatorv1.Mul_Response, error) {
	out, err := c.impl.Mul(ctx, req.A, req.B)
	if err != nil {
		return nil, err
	}
	return &calculatorv1.Mul_Response{C: out}, nil
}

func (c *calculatorServer) Div(ctx context.Context, req *calculatorv1.Div_Request) (*calculatorv1.Div_Response, error) {
	out, err := c.impl.Div(ctx, req.A, req.B)
	if err != nil {
		return nil, err
	}
	return &calculatorv1.Div_Response{C: out}, nil
}
