//go:build wasm

package main

import (
	"context"
	"fmt"

	"github.com/lovromazgon/hornet"
	"github.com/lovromazgon/hornet/examples/calculator/sdk"
)

func main() {
	// The main function is required by the compiler, but will never actually
	// be called, so it can stay empty.
}

func init() {
	// The plugin is initialized in init.
	srv := hornet.NewServer()
	sdk.RegisterCalculator(srv, &Calculator{})
	hornet.InitPlugin(srv)
}

// Calculator implements the interface defined in the SDK.
var _ sdk.Calculator = (*Calculator)(nil)

type Calculator struct{}

func (c *Calculator) Add(ctx context.Context, a, b int64) (int64, error) {
	r := a + b
	fmt.Printf("[PLUGIN] %d + %d = %d\n", a, b, r)
	return r, nil
}

func (c *Calculator) Sub(ctx context.Context, a, b int64) (int64, error) {
	r := a - b
	fmt.Printf("[PLUGIN] %d - %d = %d\n", a, b, r)
	return r, nil
}

func (c *Calculator) Mul(ctx context.Context, a, b int64) (int64, error) {
	r := a * b
	fmt.Printf("[PLUGIN] %d * %d = %d\n", a, b, r)
	return r, nil
}

func (c *Calculator) Div(ctx context.Context, a, b int64) (int64, error) {
	if b == 0 {
		fmt.Println("[PLUGIN] Whoah, don't divide by 0!")
		return 0, sdk.ErrDivisionByZero
	}
	r := a / b
	fmt.Printf("[PLUGIN] %d / %d = %d\n", a, b, r)
	return r, nil
}
