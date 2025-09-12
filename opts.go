package hornet

import "log/slog"

// ClientOption configures the client.
type ClientOption interface {
	applyClient(opt *clientOptions)
}

// clientOptionFunc wraps a function that modifies clientOptions into an
// implementation of the ClientOption interface.
type clientOptionFunc func(*clientOptions)

func (f clientOptionFunc) applyClient(opt *clientOptions) { f(opt) }

// ServerOption configures the server.
type ServerOption interface {
	applyServer(opt *serverOptions)
}

// serverOptionFunc wraps a function that modifies serverOptions into an
// implementation of the ServerOption interface.
type serverOptionFunc func(*serverOptions)

func (f serverOptionFunc) applyServer(opt *serverOptions) { f(opt) }

// ClientServerOption is an option that can configure both the Server and
// Client.
type ClientServerOption interface {
	ClientOption
	ServerOption
}

// clientServerOptionFunc wraps a clientOptionFunc and serverOptionFunc to
// create a combined option applicable to both the server and client.
type clientServerOptionFunc struct {
	clientOptionFunc
	serverOptionFunc
}

// WithLogger returns a ClientServerOption that can set the logger for the
// server or the client.
func WithLogger(l *slog.Logger) ClientServerOption {
	return clientServerOptionFunc{
		clientOptionFunc: func(opt *clientOptions) { opt.logger = l },
		serverOptionFunc: func(opt *serverOptions) { opt.logger = l },
	}
}
