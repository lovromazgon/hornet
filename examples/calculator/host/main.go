package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/lovromazgon/hornet/examples/calculator/sdk"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

const pluginPath = "../plugin/main.wasm"

type operation string

const (
	opAdd operation = "+"
	opSub operation = "-"
	opMul operation = "*"
	opDiv operation = "/"
)

func (o operation) String() string {
	switch o {
	case opAdd:
		return "Add"
	case opSub:
		return "Sub"
	case opMul:
		return "Mul"
	case opDiv:
		return "Div"
	default:
		return ""
	}
}

func main() {
	ctx := context.Background()

	calc, closeFn, err := initPlugin(ctx, pluginPath)
	if err != nil {
		panic(err)
	}
	defer closeFn()

	// Simple REPL to read operations from stdin.
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Check for quit commands
		if input == "q" || input == "quit" || input == "exit" {
			break
		}

		// Try to parse arithmetic operations
		op, ok := parseOperation(input)
		if !ok {
			fmt.Println("Error: Invalid operation")
			continue
		}

		parts := strings.Split(input, string(op))
		if len(parts) != 2 {
			fmt.Println("Error: Invalid format")
			continue
		}

		a, err1 := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
		b, err2 := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)

		if err1 != nil || err2 != nil {
			fmt.Println("Error: Invalid numbers")
			continue
		}

		// Perform the operation using the plugin
		var c int64
		switch op {
		case opAdd:
			c, err = calc.Add(ctx, a, b)
		case opSub:
			c, err = calc.Sub(ctx, a, b)
		case opMul:
			c, err = calc.Mul(ctx, a, b)
		case opDiv:
			c, err = calc.Div(ctx, a, b)
			if errors.Is(err, sdk.ErrDivisionByZero) {
				fmt.Println("[ HOST ] Error: detected sdk.ErrDivisionByZero error")
				continue
			}
		}

		if err != nil {
			fmt.Printf("[ HOST ] Error: %v\n", err)
			continue
		}
		fmt.Printf("[ HOST ] %s(%d, %d) = %d\n", op, a, b, c)
	}
}

func parseOperation(input string) (operation, bool) {
	switch {
	case strings.Contains(input, "+"):
		return opAdd, true
	case strings.Contains(input, "-"):
		return opSub, true
	case strings.Contains(input, "*"):
		return opMul, true
	case strings.Contains(input, "/"):
		return opDiv, true
	default:
		return "", false
	}
}

func initPlugin(ctx context.Context, path string) (sdk.Calculator, func(), error) {
	// Create a Wasm runtime, set up WASI.
	r := wazero.NewRuntime(ctx)

	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	wasmBytes, err := os.ReadFile(path)
	if err != nil {
		r.Close(ctx)
		return nil, nil, fmt.Errorf("failed to read Wasm file %q: %w", path, err)
	}

	initStop, initDone := initProgress()

	module, calc, err := sdk.InitializeModuleAndCalculator(ctx, r, wasmBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to instantiate Wasm module: %w", err)
	}

	close(initStop)
	<-initDone

	closeFn := func() {
		fmt.Println("Closing Wasm module...")
		err := module.Close(ctx)
		if err != nil {
			fmt.Printf("Error closing module: %v\n", err)
		}
		fmt.Println("Closing Wasm runtime...")
		err = r.Close(ctx)
		if err != nil {
			fmt.Printf("Error closing runtime: %v\n", err)
		}
	}

	return calc, closeFn, nil
}

func initProgress() (chan<- struct{}, <-chan struct{}) {
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)

		now := time.Now()
		ticker := time.Tick(100 * time.Millisecond)
		progressChars := []string{"-", "\\", "|", "/"}
		charIndex := 0

		fmt.Print("Initializing Wasm module...  ")
		for {
			select {
			case <-stop:
				fmt.Printf("\b\b\b\b\b - Done (%s)! ✅\n", time.Since(now).Truncate(time.Millisecond))
				return
			case <-ticker:
				fmt.Printf("\b%s", progressChars[charIndex])
				charIndex = (charIndex + 1) % len(progressChars)
			}
		}
	}()
	return stop, done
}
