package http1

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/asymmetric-effort/ginger/internal/logging"
)

func TestServerHandleAndServe(t *testing.T) {
	s := NewServer()
	s.Handle("GET", "/hello", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	go func() {
		s.ListenAndServe(addr, nil)
	}()
	time.Sleep(50 * time.Millisecond)
	defer s.GracefulShutdown(time.Second)

	resp, err := http.Get("http://" + addr + "/hello")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello" {
		t.Errorf("body = %q", body)
	}
}

func TestServerMethodNotAllowed(t *testing.T) {
	s := NewServer()
	s.Handle("POST", "/submit", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	go func() {
		s.ListenAndServe(addr, nil)
	}()
	time.Sleep(50 * time.Millisecond)
	defer s.GracefulShutdown(time.Second)

	resp, err := http.Get("http://" + addr + "/submit")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

func TestServerHandleFunc(t *testing.T) {
	s := NewServer()
	s.HandleFunc("/any", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("any method"))
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	go func() {
		s.ListenAndServe(addr, nil)
	}()
	time.Sleep(50 * time.Millisecond)
	defer s.GracefulShutdown(time.Second)

	resp, err := http.Post("http://"+addr+"/any", "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d", resp.StatusCode)
	}
}

func TestServerShutdown(t *testing.T) {
	s := NewServer()
	s.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	go func() {
		s.ListenAndServe(addr, nil)
	}()
	time.Sleep(50 * time.Millisecond)

	err = s.Shutdown(context.Background())
	if err != nil {
		t.Errorf("Shutdown error: %v", err)
	}
}

func TestServerAddr(t *testing.T) {
	s := NewServer()
	if s.Addr() != "" {
		t.Error("Addr before listen should be empty")
	}
}

func TestListenAndServeInvalidAddr(t *testing.T) {
	s := NewServer()
	err := s.ListenAndServe("invalid:addr:port:too:many", nil)
	if err == nil {
		t.Error("expected error for invalid address")
	}
}

// === Middleware Tests ===

func TestRequestIDMiddleware(t *testing.T) {
	mw := RequestIDMiddleware()
	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	handler(w, r)

	id := w.Header().Get("X-Request-ID")
	if id == "" {
		t.Error("missing X-Request-ID")
	}
	if len(id) != 16 { // 8 bytes = 16 hex chars
		t.Errorf("X-Request-ID length = %d", len(id))
	}
}

func TestLoggingMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.New(logging.Config{Level: logging.LevelDebug, Output: &buf, Format: "json"})

	mw := LoggingMiddleware(logger)
	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/spans", nil)
	handler(w, r)

	output := buf.String()
	if !strings.Contains(output, "http request") {
		t.Errorf("missing log: %s", output)
	}
	if !strings.Contains(output, "POST") {
		t.Errorf("missing method: %s", output)
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.New(logging.Config{Level: logging.LevelDebug, Output: &buf, Format: "json"})

	mw := RecoveryMiddleware(logger)
	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	handler(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	if !strings.Contains(buf.String(), "panic recovered") {
		t.Error("missing panic log")
	}
}

func TestRecoveryMiddlewareNoPanic(t *testing.T) {
	logger := logging.Nop()
	mw := RecoveryMiddleware(logger)
	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	handler(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestRecoveryMiddlewareHeaderAlreadyWritten(t *testing.T) {
	logger := logging.Nop()
	mw := RecoveryMiddleware(logger)
	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK, wroteHeader: true}
		_ = rw
		// Panic after conceptual header write
		panic("late panic")
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	handler(w, r)
	// Should not double-write headers; just verify no second panic
}

func TestTimeoutMiddleware(t *testing.T) {
	mw := TimeoutMiddleware(50 * time.Millisecond)
	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Write([]byte("too late"))
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	handler(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}

func TestRequestSizeLimitMiddleware(t *testing.T) {
	mw := RequestSizeLimitMiddleware(10)
	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Small body — OK
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", strings.NewReader("small"))
	handler(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("small body: status = %d", w.Code)
	}

	// Large body — rejected
	w = httptest.NewRecorder()
	r = httptest.NewRequest("POST", "/", strings.NewReader(strings.Repeat("x", 100)))
	handler(w, r)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("large body: status = %d, want 413", w.Code)
	}
}

func TestCORSMiddleware(t *testing.T) {
	mw := CORSMiddleware([]string{"http://example.com"})
	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Matching origin
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Origin", "http://example.com")
	handler(w, r)
	if w.Header().Get("Access-Control-Allow-Origin") != "http://example.com" {
		t.Error("CORS origin not set")
	}

	// Non-matching origin
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Origin", "http://evil.com")
	handler(w, r)
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("should not set CORS for non-matching origin")
	}

	// OPTIONS preflight
	w = httptest.NewRecorder()
	r = httptest.NewRequest("OPTIONS", "/", nil)
	r.Header.Set("Origin", "http://example.com")
	handler(w, r)
	if w.Code != http.StatusNoContent {
		t.Errorf("OPTIONS status = %d, want 204", w.Code)
	}
}

func TestCORSWildcard(t *testing.T) {
	mw := CORSMiddleware([]string{"*"})
	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Origin", "http://anything.com")
	handler(w, r)
	if w.Header().Get("Access-Control-Allow-Origin") != "http://anything.com" {
		t.Error("wildcard CORS should match any origin")
	}
}

func TestMiddlewareChain(t *testing.T) {
	s := NewServer()
	s.Use(RequestIDMiddleware())

	var buf bytes.Buffer
	logger := logging.New(logging.Config{Level: logging.LevelDebug, Output: &buf, Format: "json"})
	s.Use(LoggingMiddleware(logger))

	s.Handle("GET", "/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	go func() {
		s.ListenAndServe(addr, nil)
	}()
	time.Sleep(50 * time.Millisecond)
	defer s.GracefulShutdown(time.Second)

	resp, err := http.Get("http://" + addr + "/test")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("X-Request-ID") == "" {
		t.Error("middleware chain: missing X-Request-ID")
	}
	if !strings.Contains(buf.String(), "http request") {
		t.Error("middleware chain: missing log")
	}
}

func TestResponseWriterDoubleWriteHeader(t *testing.T) {
	w := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
	rw.WriteHeader(http.StatusCreated)
	rw.WriteHeader(http.StatusBadRequest) // should be ignored
	if rw.status != http.StatusCreated {
		t.Errorf("status = %d, want 201", rw.status)
	}
}

func TestResponseWriterImplicitWrite(t *testing.T) {
	w := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
	rw.Write([]byte("data"))
	if !rw.wroteHeader {
		t.Error("Write should mark wroteHeader")
	}
}

func TestHeaderWrittenTrue(t *testing.T) {
	w := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: w, status: http.StatusOK, wroteHeader: true}
	if !headerWritten(rw) {
		t.Error("should return true when wroteHeader is true")
	}
}

func TestHeaderWrittenNonResponseWriter(t *testing.T) {
	w := httptest.NewRecorder()
	if headerWritten(w) {
		t.Error("regular ResponseWriter should return false")
	}
}

func TestGenerateRequestID(t *testing.T) {
	id := generateRequestID()
	if len(id) != 16 {
		t.Errorf("id length = %d, want 16", len(id))
	}
	// Should be unique
	id2 := generateRequestID()
	if id == id2 {
		t.Error("IDs should be unique")
	}
}
