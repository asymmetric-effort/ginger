package grpc

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/asymmetric-effort/ginger/internal/codec/protobuf"
)

const (
	maxMessageSize = 4 * 1024 * 1024 // 4MB default
)

// Message is the interface for protobuf-serializable messages.
type Message interface {
	ProtoMarshal(enc *protobuf.Encoder) error
	ProtoUnmarshal(dec *protobuf.Decoder) error
}

// UnaryHandler handles a unary RPC.
type UnaryHandler func(ctx context.Context, reqData []byte) ([]byte, error)

// StreamHandler handles a server-streaming RPC.
type StreamHandler func(ctx context.Context, reqData []byte, stream ServerStream) error

// ServerStream is the interface for server-streaming responses.
type ServerStream interface {
	Send(data []byte) error
}

// MethodDesc describes a single RPC method.
type MethodDesc struct {
	Name          string
	UnaryHandler  UnaryHandler
	StreamHandler StreamHandler
	IsStreaming   bool
}

// ServiceDesc describes a gRPC service.
type ServiceDesc struct {
	ServiceName string
	Methods     []MethodDesc
}

// MD is metadata (string multi-map) extracted from gRPC headers.
type MD map[string][]string

// Get returns the first value for a key, or empty string.
func (m MD) Get(key string) string {
	vals := m[strings.ToLower(key)]
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

// Set sets a metadata key-value.
func (m MD) Set(key, value string) {
	m[strings.ToLower(key)] = []string{value}
}

type mdContextKey struct{}

// MetadataFromContext extracts MD from the context.
func MetadataFromContext(ctx context.Context) MD {
	md, ok := ctx.Value(mdContextKey{}).(MD)
	if !ok {
		return MD{}
	}
	return md
}

// Server is a gRPC server built on Go's stdlib HTTP/2.
type Server struct {
	mu       sync.RWMutex
	services map[string]*ServiceDesc
	methods  map[string]*MethodDesc
	maxSize  int
	httpSrv  *http.Server
}

// NewServer creates a new gRPC server.
func NewServer() *Server {
	return &Server{
		services: make(map[string]*ServiceDesc),
		methods:  make(map[string]*MethodDesc),
		maxSize:  maxMessageSize,
	}
}

// RegisterService registers a service with the server.
func (s *Server) RegisterService(desc *ServiceDesc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.services[desc.ServiceName] = desc
	for i := range desc.Methods {
		m := &desc.Methods[i]
		fullName := "/" + desc.ServiceName + "/" + m.Name
		s.methods[fullName] = m
	}
}

// Serve starts the gRPC server on the given listener.
func (s *Server) Serve(ln net.Listener) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleGRPC)

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	s.mu.Lock()
	s.httpSrv = srv
	s.mu.Unlock()

	return srv.Serve(ln)
}

// ServeTLS starts the gRPC server with TLS.
func (s *Server) ServeTLS(ln net.Listener, tlsCfg *tls.Config) error {
	tlsCfg.NextProtos = []string{"h2"}
	tlsLn := tls.NewListener(ln, tlsCfg)
	return s.Serve(tlsLn)
}

// GracefulStop gracefully shuts down the server.
func (s *Server) GracefulStop() {
	s.mu.RLock()
	srv := s.httpSrv
	s.mu.RUnlock()
	if srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}
}

func (s *Server) handleGRPC(w http.ResponseWriter, r *http.Request) {
	if r.ProtoMajor < 2 {
		http.Error(w, "gRPC requires HTTP/2", http.StatusHTTPVersionNotSupported)
		return
	}

	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/grpc") {
		http.Error(w, "invalid content-type", http.StatusUnsupportedMediaType)
		return
	}

	s.mu.RLock()
	method, ok := s.methods[r.URL.Path]
	s.mu.RUnlock()
	if !ok {
		writeGRPCStatus(w, 12, "unknown method: "+r.URL.Path) // UNIMPLEMENTED
		return
	}

	// Extract metadata
	md := MD{}
	for k, v := range r.Header {
		md[strings.ToLower(k)] = v
	}
	ctx := context.WithValue(r.Context(), mdContextKey{}, md)

	// Handle deadline
	if timeout := r.Header.Get("Grpc-Timeout"); timeout != "" {
		d, err := parseTimeout(timeout)
		if err == nil {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, d)
			defer cancel()
		}
	}

	// Read request
	reqData, err := readMessage(r.Body, r.Header.Get("Grpc-Encoding") == "gzip", s.maxSize)
	if err != nil {
		writeGRPCStatus(w, 3, "failed to read request: "+err.Error()) // INVALID_ARGUMENT
		return
	}

	if method.IsStreaming {
		// Server streaming
		w.Header().Set("Content-Type", "application/grpc+proto")
		w.Header().Set("Trailer", "Grpc-Status, Grpc-Message")
		w.WriteHeader(http.StatusOK)

		flusher, canFlush := w.(http.Flusher)
		stream := &httpServerStream{w: w, flusher: flusher, canFlush: canFlush}
		err = method.StreamHandler(ctx, reqData, stream)
		if err != nil {
			w.Header().Set("Grpc-Status", "13") // INTERNAL
			w.Header().Set("Grpc-Message", err.Error())
		} else {
			w.Header().Set("Grpc-Status", "0")
		}
	} else {
		// Unary
		respData, err := method.UnaryHandler(ctx, reqData)
		if err != nil {
			code, msg := errorToGRPC(err)
			writeGRPCStatus(w, code, msg)
			return
		}

		w.Header().Set("Content-Type", "application/grpc+proto")
		w.Header().Set("Trailer", "Grpc-Status, Grpc-Message")
		w.WriteHeader(http.StatusOK)
		writeMessage(w, respData)
		w.Header().Set("Grpc-Status", "0")
	}
}

type httpServerStream struct {
	w        http.ResponseWriter
	flusher  http.Flusher
	canFlush bool
}

func (s *httpServerStream) Send(data []byte) error {
	if err := writeMessage(s.w, data); err != nil {
		return err
	}
	if s.canFlush {
		s.flusher.Flush()
	}
	return nil
}

// readMessage reads a gRPC length-prefixed message from the body.
func readMessage(body io.Reader, compressed bool, maxSize int) ([]byte, error) {
	// gRPC message format: 1 byte compressed flag + 4 byte length + payload
	header := make([]byte, 5)
	if _, err := io.ReadFull(body, header); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	isCompressed := header[0] == 1
	length := binary.BigEndian.Uint32(header[1:5])

	if int(length) > maxSize {
		return nil, fmt.Errorf("message size %d exceeds max %d", length, maxSize)
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(body, payload); err != nil {
		return nil, fmt.Errorf("read payload: %w", err)
	}

	if isCompressed || compressed {
		reader, err := gzip.NewReader(bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("gzip: %w", err)
		}
		defer reader.Close()
		decompressed, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("decompress: %w", err)
		}
		return decompressed, nil
	}

	return payload, nil
}

// writeMessage writes a gRPC length-prefixed message.
func writeMessage(w io.Writer, data []byte) error {
	header := make([]byte, 5)
	header[0] = 0 // not compressed
	binary.BigEndian.PutUint32(header[1:5], uint32(len(data)))
	if _, err := w.Write(header); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

func writeGRPCStatus(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/grpc+proto")
	w.Header().Set("Grpc-Status", fmt.Sprintf("%d", code))
	w.Header().Set("Grpc-Message", msg)
	w.WriteHeader(http.StatusOK)
}

func parseTimeout(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid timeout")
	}
	unit := s[len(s)-1]
	val := s[:len(s)-1]
	var d time.Duration
	var n int
	_, err := fmt.Sscanf(val, "%d", &n)
	if err != nil {
		return 0, err
	}
	switch unit {
	case 'n':
		d = time.Duration(n) * time.Nanosecond
	case 'u':
		d = time.Duration(n) * time.Microsecond
	case 'm':
		d = time.Duration(n) * time.Millisecond
	case 'S':
		d = time.Duration(n) * time.Second
	case 'M':
		d = time.Duration(n) * time.Minute
	case 'H':
		d = time.Duration(n) * time.Hour
	default:
		return 0, fmt.Errorf("unknown unit %c", unit)
	}
	return d, nil
}

// StatusError is an error with a gRPC status code.
type StatusError struct {
	Code    int
	Message string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("rpc error: code = %d desc = %s", e.Code, e.Message)
}

// NewStatusError creates a StatusError.
func NewStatusError(code int, msg string) *StatusError {
	return &StatusError{Code: code, Message: msg}
}

func errorToGRPC(err error) (int, string) {
	var se *StatusError
	if ok := asStatusError(err, &se); ok {
		return se.Code, se.Message
	}
	return 13, err.Error() // INTERNAL
}

func asStatusError(err error, target **StatusError) bool {
	for err != nil {
		if se, ok := err.(*StatusError); ok {
			*target = se
			return true
		}
		// Try unwrap
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
