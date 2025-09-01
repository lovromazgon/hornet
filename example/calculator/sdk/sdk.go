package sdk

type Calculator interface {
	Add(a, b int64) int64
	Sub(a, b int64) int64
	Mul(a, b int64) int64
	Div(a, b int64) int64
}
