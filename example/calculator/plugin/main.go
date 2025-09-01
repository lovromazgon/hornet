package main

import (
	"context"
	"fmt"

	"github.com/lovromazgon/hornet/example/calculator/sdk"
	"github.com/lovromazgon/hornet/grpc"
	"github.com/lovromazgon/hornet/wasm"
)

func main() {
	// The main function is required by the compiler, but will never actually
	// be called, so it can stay empty.
}

func init() {
	// The plugin is initialized in init.
	srv := grpc.NewServer()
	sdk.RegisterCalculatorServer(srv, &Calculator{})
	wasm.Init(srv)
}

// Calculator implements the interface defined in the SDK.
var _ sdk.Calculator = (*Calculator)(nil)

type Calculator struct{}

func (c *Calculator) Add(ctx context.Context, a, b int64) (int64, error) {
	r := a + b
	fmt.Printf("%d + %d = %d\n", a, b, r)
	return r, nil
}

func (c *Calculator) Sub(ctx context.Context, a, b int64) (int64, error) {
	r := a - b
	fmt.Printf("%d - %d = %d\n", a, b, r)
	return r, nil
}

func (c *Calculator) Mul(ctx context.Context, a, b int64) (int64, error) {
	r := a * b
	fmt.Printf("%d * %d = %d\n", a, b, r)
	return r, nil
}

func (c *Calculator) Div(ctx context.Context, a, b int64) (int64, error) {
	r := a / b
	fmt.Printf("%d / %d = %d\n", a, b, r)
	return r, nil
}
