package grpc

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestServerUnaryRPC(t *testing.T) {
	srv := NewServer()
	srv.RegisterService(&ServiceDesc{
		ServiceName: "test.Service",
		Methods: []MethodDesc{{
			Name: "Echo",
			UnaryHandler: func(ctx context.Context, reqData []byte) ([]byte, error) {
				return reqData, nil // echo
			},
		}},
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	go srv.Serve(ln)
	defer srv.GracefulStop()

	// Make HTTP/1.1 request to test the handler (HTTP/2 needs TLS in stdlib)
	addr := ln.Addr().String()
	time.Sleep(50 * time.Millisecond)

	// Build gRPC request
	var body bytes.Buffer
	writeMessage(&body, []byte("hello"))

	req, _ := http.NewRequest("POST", "http://"+addr+"/test.Service/Echo", &body)
	req.Header.Set("Content-Type", "application/grpc+proto")
	req.ProtoMajor = 2 // Simulate HTTP/2

	// Use httptest recorder to test handler directly
	w := httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/test.Service/Echo", &body)
	req.Header.Set("Content-Type", "application/grpc+proto")
	req.ProtoMajor = 2
	srv.handleGRPC(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
	if w.Header().Get("Grpc-Status") != "0" {
		t.Errorf("grpc-status = %q", w.Header().Get("Grpc-Status"))
	}

	// Verify response body has gRPC frame
	respBody := w.Body.Bytes()
	if len(respBody) < 5 {
		t.Fatalf("response too short: %d", len(respBody))
	}
	payload := respBody[5:]
	if string(payload) != "hello" {
		t.Errorf("echo = %q", payload)
	}
}

func TestServerStreamingRPC(t *testing.T) {
	srv := NewServer()
	srv.RegisterService(&ServiceDesc{
		ServiceName: "test.Service",
		Methods: []MethodDesc{{
			Name:       "StreamEcho",
			IsStreaming: true,
			StreamHandler: func(ctx context.Context, reqData []byte, stream ServerStream) error {
				stream.Send([]byte("msg1"))
				stream.Send([]byte("msg2"))
				return nil
			},
		}},
	})

	var body bytes.Buffer
	writeMessage(&body, []byte("request"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test.Service/StreamEcho", &body)
	req.Header.Set("Content-Type", "application/grpc+proto")
	req.ProtoMajor = 2
	srv.handleGRPC(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}

	// Should have multiple gRPC frames in body
	respBody := w.Body.Bytes()
	if len(respBody) < 10 {
		t.Errorf("response too short for 2 messages: %d", len(respBody))
	}
}

func TestServerUnknownMethod(t *testing.T) {
	srv := NewServer()

	var body bytes.Buffer
	writeMessage(&body, []byte("req"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/unknown.Service/Method", &body)
	req.Header.Set("Content-Type", "application/grpc+proto")
	req.ProtoMajor = 2
	srv.handleGRPC(w, req)

	if w.Header().Get("Grpc-Status") != "12" {
		t.Errorf("expected UNIMPLEMENTED (12), got %q", w.Header().Get("Grpc-Status"))
	}
}

func TestServerInvalidContentType(t *testing.T) {
	srv := NewServer()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test/Method", nil)
	req.Header.Set("Content-Type", "application/json")
	req.ProtoMajor = 2
	srv.handleGRPC(w, req)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("status = %d, want 415", w.Code)
	}
}

func TestServerHTTP1Rejected(t *testing.T) {
	srv := NewServer()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test/Method", nil)
	req.Header.Set("Content-Type", "application/grpc+proto")
	req.ProtoMajor = 1
	srv.handleGRPC(w, req)

	if w.Code != http.StatusHTTPVersionNotSupported {
		t.Errorf("status = %d, want 505", w.Code)
	}
}

func TestServerHandlerError(t *testing.T) {
	srv := NewServer()
	srv.RegisterService(&ServiceDesc{
		ServiceName: "test.Service",
		Methods: []MethodDesc{{
			Name: "Fail",
			UnaryHandler: func(ctx context.Context, reqData []byte) ([]byte, error) {
				return nil, NewStatusError(5, "not found")
			},
		}},
	})

	var body bytes.Buffer
	writeMessage(&body, []byte("req"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test.Service/Fail", &body)
	req.Header.Set("Content-Type", "application/grpc+proto")
	req.ProtoMajor = 2
	srv.handleGRPC(w, req)

	if w.Header().Get("Grpc-Status") != "5" {
		t.Errorf("expected status 5, got %q", w.Header().Get("Grpc-Status"))
	}
}

func TestServerGenericError(t *testing.T) {
	srv := NewServer()
	srv.RegisterService(&ServiceDesc{
		ServiceName: "test.Service",
		Methods: []MethodDesc{{
			Name: "Fail",
			UnaryHandler: func(ctx context.Context, reqData []byte) ([]byte, error) {
				return nil, io.ErrUnexpectedEOF
			},
		}},
	})

	var body bytes.Buffer
	writeMessage(&body, []byte("req"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test.Service/Fail", &body)
	req.Header.Set("Content-Type", "application/grpc+proto")
	req.ProtoMajor = 2
	srv.handleGRPC(w, req)

	if w.Header().Get("Grpc-Status") != "13" {
		t.Errorf("expected INTERNAL (13), got %q", w.Header().Get("Grpc-Status"))
	}
}

func TestServerStreamError(t *testing.T) {
	srv := NewServer()
	srv.RegisterService(&ServiceDesc{
		ServiceName: "test.Service",
		Methods: []MethodDesc{{
			Name:       "FailStream",
			IsStreaming: true,
			StreamHandler: func(ctx context.Context, reqData []byte, stream ServerStream) error {
				return NewStatusError(13, "stream failed")
			},
		}},
	})

	var body bytes.Buffer
	writeMessage(&body, []byte("req"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test.Service/FailStream", &body)
	req.Header.Set("Content-Type", "application/grpc+proto")
	req.ProtoMajor = 2
	srv.handleGRPC(w, req)

	// Status in trailers
	if w.Code != http.StatusOK {
		t.Errorf("status = %d (should be 200 with grpc-status in trailers)", w.Code)
	}
}

func TestServerWithTimeout(t *testing.T) {
	srv := NewServer()
	srv.RegisterService(&ServiceDesc{
		ServiceName: "test.Service",
		Methods: []MethodDesc{{
			Name: "Slow",
			UnaryHandler: func(ctx context.Context, reqData []byte) ([]byte, error) {
				deadline, ok := ctx.Deadline()
				if !ok {
					t.Error("expected deadline from grpc-timeout")
				}
				if time.Until(deadline) > 10*time.Second {
					t.Error("deadline too far")
				}
				return reqData, nil
			},
		}},
	})

	var body bytes.Buffer
	writeMessage(&body, []byte("req"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test.Service/Slow", &body)
	req.Header.Set("Content-Type", "application/grpc+proto")
	req.Header.Set("Grpc-Timeout", "5S")
	req.ProtoMajor = 2
	srv.handleGRPC(w, req)
}

func TestServerMetadata(t *testing.T) {
	srv := NewServer()
	srv.RegisterService(&ServiceDesc{
		ServiceName: "test.Service",
		Methods: []MethodDesc{{
			Name: "Meta",
			UnaryHandler: func(ctx context.Context, reqData []byte) ([]byte, error) {
				md := MetadataFromContext(ctx)
				auth := md.Get("authorization")
				return []byte(auth), nil
			},
		}},
	})

	var body bytes.Buffer
	writeMessage(&body, []byte("req"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test.Service/Meta", &body)
	req.Header.Set("Content-Type", "application/grpc+proto")
	req.Header.Set("Authorization", "Bearer test-token")
	req.ProtoMajor = 2
	srv.handleGRPC(w, req)

	payload := w.Body.Bytes()[5:]
	if string(payload) != "Bearer test-token" {
		t.Errorf("metadata = %q", payload)
	}
}

func TestServerBadRequestBody(t *testing.T) {
	srv := NewServer()
	srv.RegisterService(&ServiceDesc{
		ServiceName: "test.Service",
		Methods: []MethodDesc{{
			Name: "Echo",
			UnaryHandler: func(ctx context.Context, reqData []byte) ([]byte, error) {
				return reqData, nil
			},
		}},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test.Service/Echo", strings.NewReader("too short"))
	req.Header.Set("Content-Type", "application/grpc+proto")
	req.ProtoMajor = 2
	srv.handleGRPC(w, req)

	if w.Header().Get("Grpc-Status") != "3" {
		t.Errorf("expected INVALID_ARGUMENT (3), got %q", w.Header().Get("Grpc-Status"))
	}
}

func TestParseTimeout(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
		err   bool
	}{
		{"5S", 5 * time.Second, false},
		{"100m", 100 * time.Millisecond, false},
		{"1000u", 1000 * time.Microsecond, false},
		{"1000n", 1000 * time.Nanosecond, false},
		{"2M", 2 * time.Minute, false},
		{"1H", 1 * time.Hour, false},
		{"x", 0, true},
		{"5X", 0, true},
		{"", 0, true},
	}
	for _, tt := range tests {
		d, err := parseTimeout(tt.input)
		if tt.err {
			if err == nil {
				t.Errorf("parseTimeout(%q) should error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseTimeout(%q): %v", tt.input, err)
			continue
		}
		if d != tt.want {
			t.Errorf("parseTimeout(%q) = %v, want %v", tt.input, d, tt.want)
		}
	}
}

func TestMD(t *testing.T) {
	md := MD{}
	md.Set("key", "value")
	if md.Get("key") != "value" {
		t.Error("Get")
	}
	if md.Get("missing") != "" {
		t.Error("Get missing")
	}
}

func TestStatusError(t *testing.T) {
	err := NewStatusError(5, "not found")
	if err.Error() != "rpc error: code = 5 desc = not found" {
		t.Errorf("Error() = %q", err.Error())
	}
}

func TestMetadataFromContextEmpty(t *testing.T) {
	md := MetadataFromContext(context.Background())
	if len(md) != 0 {
		t.Error("empty context should return empty MD")
	}
}

func TestReadMessageTooLarge(t *testing.T) {
	var buf bytes.Buffer
	writeMessage(&buf, make([]byte, 100))
	_, err := readMessage(&buf, false, 10) // max 10 bytes
	if err == nil {
		t.Error("expected error for oversized message")
	}
}

func TestWriteAndReadMessage(t *testing.T) {
	var buf bytes.Buffer
	err := writeMessage(&buf, []byte("test data"))
	if err != nil {
		t.Fatal(err)
	}

	data, err := readMessage(&buf, false, maxMessageSize)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "test data" {
		t.Errorf("data = %q", data)
	}
}

func TestParseInt(t *testing.T) {
	if parseInt("42") != 42 {
		t.Error("parseInt(42)")
	}
	if parseInt("0") != 0 {
		t.Error("parseInt(0)")
	}
	if parseInt("abc") != 0 {
		t.Error("parseInt(abc)")
	}
}

func TestClientConnBasic(t *testing.T) {
	cc := NewClientConn("localhost:50051")
	if cc.scheme != "http" {
		t.Errorf("scheme = %q", cc.scheme)
	}
	cc.Close()
}

func TestClientConnWithTLS(t *testing.T) {
	cc := NewClientConn("localhost:50051", WithTLS(&tls.Config{}))
	if cc.scheme != "https" {
		t.Errorf("scheme = %q", cc.scheme)
	}
	cc.Close()
}

func TestClientConnWithMaxRetries(t *testing.T) {
	cc := NewClientConn("localhost:50051", WithMaxRetries(5))
	if cc.opts.maxRetries != 5 {
		t.Errorf("maxRetries = %d", cc.opts.maxRetries)
	}
	cc.Close()
}

func TestAsStatusError(t *testing.T) {
	var se *StatusError
	if asStatusError(nil, &se) {
		t.Error("nil should not match")
	}
	err := NewStatusError(5, "test")
	if !asStatusError(err, &se) {
		t.Error("should match StatusError")
	}
	if se.Code != 5 {
		t.Errorf("code = %d", se.Code)
	}
}

func TestServerInvalidTimeout(t *testing.T) {
	srv := NewServer()
	srv.RegisterService(&ServiceDesc{
		ServiceName: "test.Service",
		Methods: []MethodDesc{{
			Name: "Echo",
			UnaryHandler: func(ctx context.Context, reqData []byte) ([]byte, error) {
				return reqData, nil
			},
		}},
	})

	var body bytes.Buffer
	writeMessage(&body, []byte("req"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test.Service/Echo", &body)
	req.Header.Set("Content-Type", "application/grpc+proto")
	req.Header.Set("Grpc-Timeout", "invalid")
	req.ProtoMajor = 2
	srv.handleGRPC(w, req)

	// Should still succeed — invalid timeout is ignored
	if w.Header().Get("Grpc-Status") != "0" {
		t.Errorf("status = %q (should succeed despite bad timeout)", w.Header().Get("Grpc-Status"))
	}
}
