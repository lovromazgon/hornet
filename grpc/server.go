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

	err := s.register(sd, ss)
	if err != nil {
		s.opts.logger.Error("proto: Server.RegisterService failed", "error", err)
		os.Exit(1) // That's what the original gRPC implementation does.
	}
}

func (s *Server) register(sd *grpc.ServiceDesc, ss any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.opts.logger.Debug("Registering service", "service", sd.ServiceName)

	if _, ok := s.services[sd.ServiceName]; ok {
		return fmt.Errorf("found duplicate service registration: %q", sd.ServiceName)
	}

	if len(sd.Streams) > 0 {
		s.opts.logger.Warn("proto: Server.RegisterService found stream service, "+
			"streams are not supported in Wasm plugins", "service", sd.ServiceName)
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

	return nil
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

	// NB: We overwrite the request bytes to reuse the same bytes buffer and
	// possibly avoid allocations.
	respBytes, err := protoMarshalAppend(reqBytes[:0], resp)
	if err != nil {
		return s.handleError(
			status.New(codes.Internal, "error marshalling response"),
			"service", service, "method", method, "response", resp, "error", err,
		)
	}

	return respBytes
}

func (s *Server) handleError(st *status.Status, args ...any) []byte {
	s.opts.logger.Debug("ERROR: proto: Server.Handle "+st.Message(), args...)

	// The first byte tells the client if it's an error or a valid response.
	out, err := proto.MarshalOptions{}.MarshalAppend([]byte{1}, st.Proto())
	if err != nil {
		panic(err)
	}

	return out
}

func protoMarshalAppend(data []byte, v any) ([]byte, error) {
	msg, ok := v.(proto.Message)
	if !ok {
		return data, fmt.Errorf("proto: error marshalling data: expected proto.Message, got %T", v)
	}

	// The first byte tells the client if it's an error or a valid response.
	data = append(data, 0)
	data, err := proto.MarshalOptions{}.MarshalAppend(data, msg)
	if err != nil {
		return data, fmt.Errorf("proto: error marshalling data: %w", err)
	}

	return data, nil
}

func protoUnmarshal(data []byte, v any) error {
	msg, ok := v.(proto.Message)
	if !ok {
		return fmt.Errorf("proto: error unmarshalling data: expected proto.Message, got %T", v)
	}

	err := proto.Unmarshal(data, msg)
	if err != nil {
		return fmt.Errorf("proto: error unmarshalling data: %w", err)
	}

	return nil
}
