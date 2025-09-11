package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"sync"

	"github.com/lovromazgon/hornet/examples/calculator/sdk"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

const path = "../plugin/main.wasm"

func main() {
	ctx := context.Background()

	// Create a Wasm runtime, set up WASI.
	r := wazero.NewRuntime(ctx)
	defer r.Close(ctx)

	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	wasmBytes, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Errorf("failed to read Wasm file %q: %w", path, err))
	}

	module, calc, err := sdk.InitializeModuleAndCalculator(ctx, r, wasmBytes)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate Wasm module: %w", err))
	}
	defer module.Close(ctx)

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			add(ctx, calc)
			sub(ctx, calc)
			mul(ctx, calc)
			div(ctx, calc)
		}()
	}

	wg.Wait()
}

func add(ctx context.Context, calc sdk.Calculator) {
	a, b := randomNumbers()

	c, err := calc.Add(ctx, a, b)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Add(%d, %d): %d\n", a, b, c)
}

func sub(ctx context.Context, calc sdk.Calculator) {
	a, b := randomNumbers()

	c, err := calc.Sub(ctx, a, b)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Sub(%d, %d): %d\n", a, b, c)
}

func mul(ctx context.Context, calc sdk.Calculator) {
	a, b := randomNumbers()

	c, err := calc.Mul(ctx, a, b)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Mul(%d, %d): %d\n", a, b, c)
}

func div(ctx context.Context, calc sdk.Calculator) {
	a, b := randomNumbers()
	if b > 50 {
		// 50% chance to get division by zero error
		b = 0
	}

	c, err := calc.Div(ctx, a, b)
	if err != nil {
		if errors.Is(err, sdk.ErrDivisionByZero) {
			fmt.Printf("Div(%d, %d) error: %v\n", a, b, err)
			return
		}
		panic(err)
	}

	fmt.Printf("Div(%d, %d): %d\n", a, b, c)
}

func randomNumbers() (int64, int64) {
	a, b := rand.Intn(100), rand.Intn(100)
	return int64(a), int64(b)
}
