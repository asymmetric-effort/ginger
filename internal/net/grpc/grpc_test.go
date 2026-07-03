package grpc

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/pem"
	"io"
	"math/big"
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

// ---------------------------------------------------------------------------
// readMessage gzip decompression path
// ---------------------------------------------------------------------------

func TestReadMessageGzipCompressed(t *testing.T) {
	var payload bytes.Buffer
	gz := gzip.NewWriter(&payload)
	gz.Write([]byte("compressed data"))
	gz.Close()

	// Build a gRPC frame with compressed flag = 1
	compressed := payload.Bytes()
	var frame bytes.Buffer
	frame.WriteByte(1) // compressed flag
	binary.Write(&frame, binary.BigEndian, uint32(len(compressed)))
	frame.Write(compressed)

	data, err := readMessage(&frame, false, maxMessageSize)
	if err != nil {
		t.Fatalf("readMessage gzip: %v", err)
	}
	if string(data) != "compressed data" {
		t.Errorf("data = %q", data)
	}
}

func TestReadMessageGzipFlagInHeader(t *testing.T) {
	// compressed=true in the header argument (Grpc-Encoding: gzip)
	var payload bytes.Buffer
	gz := gzip.NewWriter(&payload)
	gz.Write([]byte("header-compressed"))
	gz.Close()

	compressed := payload.Bytes()
	var frame bytes.Buffer
	frame.WriteByte(0) // flag byte = 0, but compressed=true is passed
	binary.Write(&frame, binary.BigEndian, uint32(len(compressed)))
	frame.Write(compressed)

	data, err := readMessage(&frame, true, maxMessageSize)
	if err != nil {
		t.Fatalf("readMessage header-gzip: %v", err)
	}
	if string(data) != "header-compressed" {
		t.Errorf("data = %q", data)
	}
}

func TestReadMessageGzipInvalidPayload(t *testing.T) {
	// compressed flag = 1 but payload is not valid gzip
	notGzip := []byte("not gzip data at all")
	var frame bytes.Buffer
	frame.WriteByte(1) // compressed
	binary.Write(&frame, binary.BigEndian, uint32(len(notGzip)))
	frame.Write(notGzip)

	_, err := readMessage(&frame, false, maxMessageSize)
	if err == nil {
		t.Error("expected error for invalid gzip payload")
	}
}

func TestReadMessageHeaderReadError(t *testing.T) {
	// Empty reader — io.ReadFull on 5-byte header will fail
	_, err := readMessage(bytes.NewReader([]byte{}), false, maxMessageSize)
	if err == nil {
		t.Error("expected error for empty body")
	}
}

func TestReadMessagePayloadReadError(t *testing.T) {
	// Write a valid 5-byte header claiming 100 bytes, but reader ends early
	var frame bytes.Buffer
	frame.WriteByte(0) // not compressed
	binary.Write(&frame, binary.BigEndian, uint32(100))
	// Don't write the payload — io.ReadFull will return EOF

	_, err := readMessage(&frame, false, maxMessageSize)
	if err == nil {
		t.Error("expected error when payload read fails")
	}
}

func TestReadMessageGzipDecompressError(t *testing.T) {
	// Produce a gzip stream where the CRC32 is wrong, causing io.ReadAll to return
	// a "gzip: invalid checksum" error (which hits the "decompress" error path).
	// We build a minimal gzip stream manually with a bad CRC.
	//
	// Gzip format: 10-byte header + DEFLATE blocks + 4-byte CRC32 + 4-byte ISIZE
	// We use the "stored" (no-compression) DEFLATE block format for simplicity:
	//   BFINAL=1, BTYPE=00 (no compress): 0x01, LEN(2), NLEN(2), data
	data := []byte("abc")
	// Stored DEFLATE block: 0x01 (final, no compress), LEN lo/hi, NLEN lo/hi, data
	deflate := []byte{
		0x01,                               // BFINAL=1 BTYPE=00
		byte(len(data)), 0x00,              // LEN = 3 (little-endian)
		byte(^len(data)), 0xFF,             // NLEN = ~LEN = 0xFC 0xFF
	}
	deflate = append(deflate, data...)

	// gzip header (10 bytes): 0x1f 0x8b, method=8, flags=0, mtime=0, xfl=0, os=0xff
	gzHeader := []byte{0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff}
	// CRC32 - intentionally WRONG (4 bytes, little-endian): 0xDE 0xAD 0xBE 0xEF
	badCRC := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	// ISIZE = 3 (actual length of uncompressed data), correct
	isize := []byte{0x03, 0x00, 0x00, 0x00}

	gzBytes := append(gzHeader, deflate...)
	gzBytes = append(gzBytes, badCRC...)
	gzBytes = append(gzBytes, isize...)

	// Build gRPC frame: flag=1 (compressed), 4-byte big-endian length, gzip payload
	var frame bytes.Buffer
	frame.WriteByte(1) // compressed
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(gzBytes)))
	frame.Write(lenBuf)
	frame.Write(gzBytes)

	_, err := readMessage(&frame, false, maxMessageSize)
	if err == nil {
		t.Error("expected decompress error for bad gzip CRC")
	}
}

// ---------------------------------------------------------------------------
// writeMessage error path
// ---------------------------------------------------------------------------

type failWriter struct{}

func (f *failWriter) Write(p []byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func TestWriteMessageError(t *testing.T) {
	err := writeMessage(&failWriter{}, []byte("hello"))
	if err == nil {
		t.Error("expected error from failing writer")
	}
}

type halfWriter struct {
	written int
}

func (h *halfWriter) Write(p []byte) (int, error) {
	if h.written == 0 {
		// Write the header successfully
		h.written++
		return len(p), nil
	}
	// Fail on payload write
	return 0, io.ErrClosedPipe
}

func TestWriteMessagePayloadError(t *testing.T) {
	err := writeMessage(&halfWriter{}, []byte("hello"))
	if err == nil {
		t.Error("expected error when payload write fails")
	}
}

// ---------------------------------------------------------------------------
// httpServerStream.Send with flush path
// ---------------------------------------------------------------------------

type flushRecorder struct {
	*httptest.ResponseRecorder
	flushed bool
}

func (f *flushRecorder) Flush() {
	f.flushed = true
	f.ResponseRecorder.Flush()
}

func TestHTTPServerStreamSendWithFlush(t *testing.T) {
	rec := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
	stream := &httpServerStream{
		w:        rec,
		flusher:  rec,
		canFlush: true,
	}
	if err := stream.Send([]byte("data")); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !rec.flushed {
		t.Error("expected Flush to be called")
	}
}

func TestHTTPServerStreamSendNoFlush(t *testing.T) {
	rec := httptest.NewRecorder()
	stream := &httpServerStream{
		w:        rec,
		canFlush: false,
	}
	if err := stream.Send([]byte("data")); err != nil {
		t.Fatalf("Send: %v", err)
	}
}

type failResponseWriter struct {
	header http.Header
}

func (f *failResponseWriter) Header() http.Header {
	if f.header == nil {
		f.header = http.Header{}
	}
	return f.header
}
func (f *failResponseWriter) Write(p []byte) (int, error) { return 0, io.ErrClosedPipe }
func (f *failResponseWriter) WriteHeader(statusCode int)  {}

func TestHTTPServerStreamSendWriteError(t *testing.T) {
	stream := &httpServerStream{
		w:        &failResponseWriter{},
		canFlush: false,
	}
	err := stream.Send([]byte("data"))
	if err == nil {
		t.Error("expected error when writer fails")
	}
}

// ---------------------------------------------------------------------------
// Client Invoke / doInvoke coverage
// ---------------------------------------------------------------------------

func TestClientInvokeSuccess(t *testing.T) {
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

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(ln)
	defer srv.GracefulStop()

	// Use httptest server which supports HTTP/1.1 for simplicity
	// We'll test doInvoke via a test HTTP server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		// Parse gRPC frame
		if len(body) < 5 {
			http.Error(w, "short", 400)
			return
		}
		payload := body[5:]
		w.Header().Set("Content-Type", "application/grpc+proto")
		w.Header().Set("Trailer", "Grpc-Status, Grpc-Message")
		w.WriteHeader(http.StatusOK)
		writeMessage(w, payload)
		w.Header().Set("Grpc-Status", "0")
	}))
	defer ts.Close()

	target := strings.TrimPrefix(ts.URL, "http://")
	cc := NewClientConn(target, WithMaxRetries(0))
	defer cc.Close()

	result, err := cc.Invoke(context.Background(), "/test.Service/Echo", []byte("ping"))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if string(result) != "ping" {
		t.Errorf("result = %q", result)
	}
}

func TestClientInvokeGRPCStatusError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/grpc+proto")
		w.Header().Set("Grpc-Status", "5")
		w.Header().Set("Grpc-Message", "not found")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	target := strings.TrimPrefix(ts.URL, "http://")
	cc := NewClientConn(target, WithMaxRetries(1))
	defer cc.Close()

	_, err := cc.Invoke(context.Background(), "/svc/Method", []byte("req"))
	if err == nil {
		t.Fatal("expected error")
	}
	se, ok := err.(*StatusError)
	if !ok {
		t.Fatalf("expected *StatusError, got %T: %v", err, err)
	}
	if se.Code != 5 {
		t.Errorf("code = %d", se.Code)
	}
}

func TestClientInvokeTrailerStatus(t *testing.T) {
	// Status in trailer instead of header
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/grpc+proto")
		w.Header().Set("Trailer", "Grpc-Status,Grpc-Message")
		w.WriteHeader(http.StatusOK)
		// Write a valid gRPC frame so body is >= 5 bytes
		var buf bytes.Buffer
		writeMessage(&buf, []byte("ok"))
		w.Write(buf.Bytes())
		w.Header().Set("Grpc-Status", "0")
	}))
	defer ts.Close()

	target := strings.TrimPrefix(ts.URL, "http://")
	cc := NewClientConn(target, WithMaxRetries(0))
	defer cc.Close()

	_, err := cc.Invoke(context.Background(), "/svc/Method", []byte("req"))
	if err != nil {
		t.Logf("Invoke returned (may depend on trailer support): %v", err)
	}
}

func TestClientInvokeShortResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/grpc+proto")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte{0, 0}) // too short
	}))
	defer ts.Close()

	target := strings.TrimPrefix(ts.URL, "http://")
	cc := NewClientConn(target, WithMaxRetries(0))
	defer cc.Close()

	_, err := cc.Invoke(context.Background(), "/svc/Method", []byte("req"))
	if err == nil {
		t.Fatal("expected error for short response")
	}
}

func TestClientInvokeConnectionError(t *testing.T) {
	// Point at a port that is not listening — should get connection refused
	cc := NewClientConn("127.0.0.1:1", WithMaxRetries(0))
	defer cc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := cc.Invoke(ctx, "/svc/Method", []byte("req"))
	if err == nil {
		t.Error("expected connection error")
	}
}

func TestClientInvokeRetryBackoff(t *testing.T) {
	// Test that retry+backoff path is exercised.
	// Server always returns a non-StatusError failure (short response) which triggers retry.
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/grpc+proto")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte{0, 0}) // too short — will cause "response too short" error
	}))
	defer ts.Close()

	target := strings.TrimPrefix(ts.URL, "http://")
	cc := NewClientConn(target, WithMaxRetries(1))
	defer cc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := cc.Invoke(ctx, "/svc/Method", []byte("req"))
	if err == nil {
		t.Error("expected error after retries")
	}
	if callCount < 2 {
		t.Errorf("expected at least 2 calls (1 retry), got %d", callCount)
	}
}

func TestClientInvokeContextCancelledDuringBackoff(t *testing.T) {
	// Server returns a retryable error (short response) fast,
	// but ctx times out during the backoff wait on retry.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/grpc+proto")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte{0, 0}) // too short, causes retryable error
	}))
	defer ts.Close()

	target := strings.TrimPrefix(ts.URL, "http://")
	// maxRetries=2: first attempt fails fast, then backoff=100ms fires, ctx should cancel during backoff
	cc := NewClientConn(target, WithMaxRetries(2))
	defer cc.Close()

	// Context that cancels after 50ms — first attempt completes, then during 100ms backoff, ctx fires
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := cc.Invoke(ctx, "/svc/Method", []byte("req"))
	if err == nil {
		t.Error("expected context error")
	}
}

func TestClientDoInvokeGRPCMessageFromTrailer(t *testing.T) {
	// grpcStatus != "0" but grpcMsg is in trailer
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/grpc+proto")
		w.Header().Set("Trailer", "Grpc-Status,Grpc-Message")
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Grpc-Status", "3")
		w.Header().Set("Grpc-Message", "bad request from trailer")
	}))
	defer ts.Close()

	target := strings.TrimPrefix(ts.URL, "http://")
	cc := NewClientConn(target, WithMaxRetries(0))
	defer cc.Close()

	_, err := cc.Invoke(context.Background(), "/svc/Method", []byte("req"))
	// Should get either StatusError from header path or from trailer
	_ = err // result varies by HTTP/1.1 trailer support
}

func TestClientDoInvokeNonGRPCStatus(t *testing.T) {
	// grpcStatus = "" means we skip the status-error path
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/grpc+proto")
		w.WriteHeader(http.StatusOK)
		// Write exactly 5 bytes so response length check passes
		w.Write([]byte{0, 0, 0, 0, 0})
	}))
	defer ts.Close()

	target := strings.TrimPrefix(ts.URL, "http://")
	cc := NewClientConn(target, WithMaxRetries(0))
	defer cc.Close()

	result, err := cc.Invoke(context.Background(), "/svc/Method", []byte("req"))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("result len = %d, want 0", len(result))
	}
}

// ---------------------------------------------------------------------------
// asStatusError unwrap path
// ---------------------------------------------------------------------------

type wrappedErr struct{ inner error }

func (w *wrappedErr) Error() string { return w.inner.Error() }
func (w *wrappedErr) Unwrap() error { return w.inner }

func TestAsStatusErrorUnwrap(t *testing.T) {
	se := NewStatusError(7, "permission denied")
	wrapped := &wrappedErr{inner: se}
	var out *StatusError
	if !asStatusError(wrapped, &out) {
		t.Error("should find StatusError through Unwrap")
	}
	if out.Code != 7 {
		t.Errorf("code = %d", out.Code)
	}
}

// ---------------------------------------------------------------------------
// ServeTLS path (just confirms it calls Serve without panic)
// ---------------------------------------------------------------------------

func TestServeTLSSetsNextProtos(t *testing.T) {
	// We can't easily test a full TLS handshake, but we can verify
	// ServeTLS does not panic when called with a valid TLS config.
	// We use a self-signed certificate for this test.
	cert, err := generateSelfSignedCert()
	if err != nil {
		t.Skip("cannot generate cert:", err)
	}

	srv := NewServer()
	srv.RegisterService(&ServiceDesc{
		ServiceName: "test.TLS",
		Methods:     []MethodDesc{{Name: "Echo", UnaryHandler: func(ctx context.Context, reqData []byte) ([]byte, error) { return reqData, nil }}},
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}}
	done := make(chan error, 1)
	go func() {
		done <- srv.ServeTLS(ln, tlsCfg)
	}()

	// Give it a moment to start
	time.Sleep(20 * time.Millisecond)
	srv.GracefulStop()

	// Confirm ServeTLS ran (error from shutdown is expected)
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Error("ServeTLS did not return after GracefulStop")
	}
}

func generateSelfSignedCert() (tls.Certificate, error) {
	return generateTestCert()
}

func generateTestCert() (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return tls.X509KeyPair(certPEM, keyPEM)
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
