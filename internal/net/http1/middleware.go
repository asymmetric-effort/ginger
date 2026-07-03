package http1

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/asymmetric-effort/ginger/internal/logging"
)

// HandlerFunc is the HTTP handler function type.
type HandlerFunc = func(http.ResponseWriter, *http.Request)

// Middleware wraps a HandlerFunc.
type Middleware func(HandlerFunc) HandlerFunc

// RequestIDMiddleware injects a unique request ID into the response header.
func RequestIDMiddleware() Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			id := generateRequestID()
			w.Header().Set("X-Request-ID", id)
			next(w, r)
		}
	}
}

// LoggingMiddleware logs each request with method, path, status, and duration.
func LoggingMiddleware(logger logging.Logger) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
			next(rw, r)
			logger.Info("http request",
				logging.String("method", r.Method),
				logging.String("path", r.URL.Path),
				logging.Int("status", rw.status),
				logging.Duration("duration", time.Since(start)),
			)
		}
	}
}

// RecoveryMiddleware catches panics and returns a 500 response.
func RecoveryMiddleware(logger logging.Logger) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered",
						logging.String("path", r.URL.Path),
						logging.String("panic", fmt.Sprintf("%v", rec)),
					)
					if !headerWritten(w) {
						http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					}
				}
			}()
			next(w, r)
		}
	}
}

// TimeoutMiddleware enforces a request timeout.
func TimeoutMiddleware(timeout time.Duration) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			http.TimeoutHandler(http.HandlerFunc(next), timeout, "Request Timeout").ServeHTTP(w, r)
		}
	}
}

// RequestSizeLimitMiddleware limits the request body size.
func RequestSizeLimitMiddleware(maxBytes int64) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next(w, r)
		}
	}
}

// CORSMiddleware adds CORS headers.
func CORSMiddleware(allowedOrigins []string) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowed := false
			for _, o := range allowedOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}
			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next(w, r)
		}
	}
}

type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.status = code
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.wroteHeader = true
	}
	return rw.ResponseWriter.Write(b)
}

func headerWritten(w http.ResponseWriter) bool {
	rw, ok := w.(*responseWriter)
	if !ok {
		return false
	}
	return rw.wroteHeader
}

func generateRequestID() string {
	b := make([]byte, 8)
	_, err := io.ReadFull(rand.Reader, b)
	if err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}
