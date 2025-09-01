package sdk

import "context"

type Calculator interface {
	Add(ctx context.Context, a, b int64) (int64, error)
	Sub(ctx context.Context, a, b int64) (int64, error)
	Mul(ctx context.Context, a, b int64) (int64, error)
	Div(ctx context.Context, a, b int64) (int64, error)
}
