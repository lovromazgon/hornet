package sdk

import (
	"context"
	"errors"
	"fmt"

	"github.com/lovromazgon/hornet"
	calculatorv1 "github.com/lovromazgon/hornet/examples/calculator/sdk/proto/calculator/v1"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var ErrDivisionByZero = errors.New("division by zero")

func InitializeModuleAndCalculator(
	ctx context.Context,
	runtime wazero.Runtime,
	source []byte,
	opts ...hornet.ClientOption,
) (api.Module, Calculator, error) {
	module, calc, err := hornet.InstantiateModuleAndClient(ctx, runtime, source, calculatorv1.NewCalculatorPluginClient, opts...)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate Wasm module: %w", err))
	}

	return module, NewCalculatorFromClient(calc), nil
}

func NewCalculatorFromClient(client calculatorv1.CalculatorPluginClient) Calculator {
	return &calculatorClient{client: client}
}

type calculatorClient struct {
	client calculatorv1.CalculatorPluginClient
}

var _ Calculator = (*calculatorClient)(nil)

func (c *calculatorClient) Add(ctx context.Context, a, b int64) (int64, error) {
	out, err := c.client.Add(ctx, &calculatorv1.AddRequest{A: a, B: b})
	if err != nil {
		return 0, err
	}

	return out.GetC(), nil
}

func (c *calculatorClient) Sub(ctx context.Context, a, b int64) (int64, error) {
	out, err := c.client.Sub(ctx, &calculatorv1.SubRequest{A: a, B: b})
	if err != nil {
		return 0, err
	}

	return out.GetC(), nil
}

func (c *calculatorClient) Mul(ctx context.Context, a, b int64) (int64, error) {
	out, err := c.client.Mul(ctx, &calculatorv1.MulRequest{A: a, B: b})
	if err != nil {
		return 0, err
	}

	return out.GetC(), nil
}

func (c *calculatorClient) Div(ctx context.Context, a, b int64) (int64, error) {
	out, err := c.client.Div(ctx, &calculatorv1.DivRequest{A: a, B: b})
	if err != nil {
		// Check if the error is a gRPC error with code InvalidArgument,
		// which indicates a division by zero attempt.
		// If so, return the predefined ErrDivisionByZero error.
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.InvalidArgument {
			return 0, ErrDivisionByZero
		}
		return 0, err
	}

	return out.GetC(), nil
}

// RegisterCalculator registers the Calculator implementation on the grpc
// service registrar (grpc.Server). Use this method when initializing the plugin.
func RegisterCalculator(srv grpc.ServiceRegistrar, calc Calculator) {
	calculatorv1.RegisterCalculatorPluginServer(srv, &calculatorServer{impl: calc})
}

// calculatorServer is a utility struct, an adapter that wraps a Calculator and
// exposes it as a server that implements the proto server definition.
type calculatorServer struct {
	calculatorv1.UnimplementedCalculatorPluginServer

	impl Calculator
}

var _ calculatorv1.CalculatorPluginServer = (*calculatorServer)(nil)

func (c *calculatorServer) Add(ctx context.Context, req *calculatorv1.AddRequest) (*calculatorv1.AddResponse, error) {
	out, err := c.impl.Add(ctx, req.GetA(), req.GetB())
	if err != nil {
		return nil, err
	}

	return &calculatorv1.AddResponse{C: out}, nil
}

func (c *calculatorServer) Sub(ctx context.Context, req *calculatorv1.SubRequest) (*calculatorv1.SubResponse, error) {
	out, err := c.impl.Sub(ctx, req.GetA(), req.GetB())
	if err != nil {
		return nil, err
	}

	return &calculatorv1.SubResponse{C: out}, nil
}

func (c *calculatorServer) Mul(ctx context.Context, req *calculatorv1.MulRequest) (*calculatorv1.MulResponse, error) {
	out, err := c.impl.Mul(ctx, req.GetA(), req.GetB())
	if err != nil {
		return nil, err
	}

	return &calculatorv1.MulResponse{C: out}, nil
}

func (c *calculatorServer) Div(ctx context.Context, req *calculatorv1.DivRequest) (*calculatorv1.DivResponse, error) {
	if req.GetB() == 0 {
		// Return a gRPC InvalidArgument error if division by zero is attempted.
		// The client can then check for this specific error and handle it
		// accordingly.
		return nil, status.Error(codes.InvalidArgument, ErrDivisionByZero.Error())
	}

	out, err := c.impl.Div(ctx, req.GetA(), req.GetB())
	if err != nil {
		return nil, err
	}

	return &calculatorv1.DivResponse{C: out}, nil
}
