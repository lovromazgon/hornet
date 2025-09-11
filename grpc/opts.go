package grpc

import "log/slog"

// ClientOption configures the client.
type ClientOption interface {
	applyClient(*clientOptions)
}

// funcClientOption wraps a function that modifies clientOptions into an
// implementation of the ClientOption interface.
type funcClientOption func(*clientOptions)

func (f funcClientOption) applyClient(so *clientOptions) { f(so) }

// ServerOption configures the server.
type ServerOption interface {
	applyServer(*serverOptions)
}

// funcServerOption wraps a function that modifies serverOptions into an
// implementation of the ServerOption interface.
type funcServerOption func(*serverOptions)

func (f funcServerOption) applyServer(so *serverOptions) { f(so) }

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
		funcClientOption: func(o *clientOptions) { o.logger = l },
		funcServerOption: func(o *serverOptions) { o.logger = l },
	}
}

func WithMaxConcurrentRequests(limit int) ClientOption {
	return funcClientOption(func(o *clientOptions) { o.maxConcurrentRequests = limit })
}
