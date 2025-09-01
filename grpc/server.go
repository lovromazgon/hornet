package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// serviceInfo wraps information about a service. It is very similar to
// grpc.ServiceDesc and is constructed from it for internal purposes.
type serviceInfo struct {
	// Contains the implementation for the methods in this service.
	serviceImpl any
	methods     map[string]*grpc.MethodDesc
}

type serverOptions struct {
	logger *slog.Logger
}

var defaultServerOptions = serverOptions{
	logger: slog.Default(),
}

type ServerOption interface {
	applyServer(*serverOptions)
}

// funcServerOption wraps a function that modifies serverOptions into an
// implementation of the ServerOption interface.
type funcServerOption func(*serverOptions)

func (f funcServerOption) applyServer(so *serverOptions) { f(so) }

// WithLogger returns a ServerOption that can set the logger for the server.
func WithLogger(l *slog.Logger) ServerOption {
	return funcServerOption(func(o *serverOptions) { o.logger = l })
}

var _ grpc.ServiceRegistrar = (*Server)(nil)

type Server struct {
	opts serverOptions

	mu       sync.Mutex // guards following fields
	services map[string]*serviceInfo
}

func NewServer(opt ...ServerOption) *Server {
	opts := defaultServerOptions
	for _, o := range opt {
		o.applyServer(&opts)
	}

	return &Server{
		opts:     opts,
		services: make(map[string]*serviceInfo),
	}
}

func (s *Server) RegisterService(sd *grpc.ServiceDesc, ss any) {
	if ss != nil {
		ht := reflect.TypeOf(sd.HandlerType).Elem()
		st := reflect.TypeOf(ss)
		if !st.Implements(ht) {
			s.opts.logger.Error("proto: Server.RegisterService found an incompatible handler type", "want", ht, "got", st)
			os.Exit(1) // That's what the original gRPC implementation does.
		}
	}
	s.register(sd, ss)
}

func (s *Server) register(sd *grpc.ServiceDesc, ss any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opts.logger.Debug("Registering service", "service", sd.ServiceName)
	// if s.serve {
	// 	logger.Fatalf("grpc: Server.RegisterService after Server.Serve for %q", sd.ServiceName)
	// }
	if _, ok := s.services[sd.ServiceName]; ok {
		s.opts.logger.Error("proto: Server.RegisterService found duplicate service registration", "service", sd.ServiceName)
		os.Exit(1)
	}
	if len(sd.Streams) > 0 {
		s.opts.logger.Warn("proto: Server.RegisterService found stream service, streams are not supported in Wasm plugins", "service", sd.ServiceName)
	}
	info := &serviceInfo{
		serviceImpl: ss,
		methods:     make(map[string]*grpc.MethodDesc),
	}
	for i := range sd.Methods {
		d := &sd.Methods[i]
		info.methods[d.MethodName] = d
	}
	s.services[sd.ServiceName] = info
}

// Handle implements the wasm.Handler interface and processes the bytes sent to
// the plugin as a gRPC request.
func (s *Server) Handle(fn string, reqBytes []byte) []byte {
	// Start a new context for each request.
	ctx := context.Background()

	pos := strings.LastIndex(fn, "/")
	if pos == -1 {
		return s.handleError(
			status.New(codes.Unimplemented, "malformed method name"),
			"method", fn,
		)
	}

	service := strings.TrimPrefix(fn[1:pos], "/")
	method := fn[pos+1:]

	srv, ok := s.services[service]
	if !ok {
		return s.handleError(
			status.New(codes.Unimplemented, "unknown service"),
			"service", service,
		)
	}
	sd, ok := srv.methods[method]
	if !ok {
		return s.handleError(
			status.New(codes.Unimplemented, "unknown method"),
			"service", service, "method", method,
		)
	}

	decFn := func(v any) error {
		return protoUnmarshal(reqBytes, v)
	}

	resp, err := sd.Handler(srv.serviceImpl, ctx, decFn, nil)
	if err != nil {
		st, ok := status.FromError(err)
		if !ok {
			st = status.FromContextError(err)
			err = st.Err()
		}
		return s.handleError(st, "service", service, "method", method, "error", err)
	}

	// TODO: reuse bytes buffer
	respBytes, err := protoMarshalAppend(nil, resp)
	if err != nil {
		return s.handleError(
			status.New(codes.Internal, "error marshalling response"),
			"service", service, "method", method, "response", resp, "error", err,
		)
	}

	return respBytes
}

func (s *Server) handleError(st *status.Status, args ...any) []byte {
	s.opts.logger.Error(fmt.Sprintf("proto: Server.Handle %s", st.Message()), args...)

	out, err := proto.Marshal(st.Proto())
	if err != nil {
		panic(err)
	}

	return out
}
