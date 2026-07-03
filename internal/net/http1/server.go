package http1

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// Server wraps net/http.Server with PQC TLS config injection and graceful shutdown.
type Server struct {
	httpServer *http.Server
	mux        *http.ServeMux
	middleware []Middleware
}

// NewServer creates a new HTTP/1.1 server.
func NewServer() *Server {
	mux := http.NewServeMux()
	return &Server{
		httpServer: &http.Server{
			ReadTimeout:       30 * time.Second,
			ReadHeaderTimeout: 10 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       120 * time.Second,
		},
		mux: mux,
	}
}

// Handle registers a handler for the given method and pattern.
func (s *Server) Handle(method, pattern string, h HandlerFunc) {
	handler := s.applyMiddleware(h)
	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if method != "" && r.Method != method {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		handler(w, r)
	})
}

// HandleFunc registers a handler for any method at the given pattern.
func (s *Server) HandleFunc(pattern string, h HandlerFunc) {
	handler := s.applyMiddleware(h)
	s.mux.HandleFunc(pattern, handler)
}

// Use adds middleware to the chain. Middleware is applied in order.
func (s *Server) Use(mw Middleware) {
	s.middleware = append(s.middleware, mw)
}

// ListenAndServe starts the server on the given address.
func (s *Server) ListenAndServe(addr string, tlsCfg *tls.Config) error {
	s.httpServer.Addr = addr
	s.httpServer.Handler = s.mux
	s.httpServer.TLSConfig = tlsCfg

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	if tlsCfg != nil {
		ln = tls.NewListener(ln, tlsCfg)
	}

	return s.httpServer.Serve(ln)
}

// Shutdown gracefully shuts down the server with a timeout.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// GracefulShutdown shuts down the server with the given timeout.
func (s *Server) GracefulShutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.Shutdown(ctx)
}

// Addr returns the server's listener address, or empty if not serving.
func (s *Server) Addr() string {
	return s.httpServer.Addr
}

func (s *Server) applyMiddleware(h HandlerFunc) HandlerFunc {
	// Apply in reverse order so first Use() is outermost
	for i := len(s.middleware) - 1; i >= 0; i-- {
		h = s.middleware[i](h)
	}
	return h
}
