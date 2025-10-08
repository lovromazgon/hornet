package sdk

import (
	"context"
	"errors"
)

// ErrDivisionByZero should be returned when attempting to divide by zero.
var ErrDivisionByZero = errors.New("division by zero")

// Calculator is the interface for the plugin.
//
// Note that it's advisable for all methods to take a context as a parameter and
// return an error, so the same interface can be used both on the host and in
// the plugin.
type Calculator interface {
	Add(ctx context.Context, a, b int64) (int64, error)
	Sub(ctx context.Context, a, b int64) (int64, error)
	Mul(ctx context.Context, a, b int64) (int64, error)
	Div(ctx context.Context, a, b int64) (int64, error)
}
