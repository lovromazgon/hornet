package grpc

import "log/slog"

// ClientOption configures the client.
type ClientOption interface {
	applyClient(opt *clientOptions)
}

// funcClientOption wraps a function that modifies clientOptions into an
// implementation of the ClientOption interface.
type funcClientOption func(*clientOptions)

func (f funcClientOption) applyClient(opt *clientOptions) { f(opt) }

// ServerOption configures the server.
type ServerOption interface {
	applyServer(opt *serverOptions)
}

// funcServerOption wraps a function that modifies serverOptions into an
// implementation of the ServerOption interface.
type funcServerOption func(*serverOptions)

func (f funcServerOption) applyServer(opt *serverOptions) { f(opt) }

// ClientServerOption is an option that can configure both the Server and
// Client.
type ClientServerOption interface {
	ClientOption
	ServerOption
}

// funcClientServerOption wraps a funcClientOption and funcServerOption to
// create a combined option applicable to both the server and client.
type funcClientServerOption struct {
	funcClientOption
	funcServerOption
}

// WithLogger returns a ClientServerOption that can set the logger for the
// server or the client.
func WithLogger(l *slog.Logger) ClientServerOption {
	return funcClientServerOption{
		funcClientOption: func(opt *clientOptions) { opt.logger = l },
		funcServerOption: func(opt *serverOptions) { opt.logger = l },
	}
}
