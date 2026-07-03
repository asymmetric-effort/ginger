package grpc

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DialOption configures a client connection.
type DialOption func(*dialOptions)

type dialOptions struct {
	tlsConfig  *tls.Config
	maxRetries int
}

// WithTLS configures TLS for the client.
func WithTLS(cfg *tls.Config) DialOption {
	return func(o *dialOptions) {
		o.tlsConfig = cfg
	}
}

// WithMaxRetries sets the max retry count.
func WithMaxRetries(n int) DialOption {
	return func(o *dialOptions) {
		o.maxRetries = n
	}
}

// ClientConn is a gRPC client connection.
type ClientConn struct {
	target     string
	opts       dialOptions
	httpClient *http.Client
	scheme     string
}

// NewClientConn creates a new gRPC client connection.
// For HTTP/2 over TLS, uses Go stdlib's built-in HTTP/2 support.
// For cleartext (testing), falls back to HTTP/1.1 framing.
func NewClientConn(target string, opts ...DialOption) *ClientConn {
	o := dialOptions{maxRetries: 3}
	for _, opt := range opts {
		opt(&o)
	}

	scheme := "http"
	transport := &http.Transport{
		ForceAttemptHTTP2: true,
	}

	if o.tlsConfig != nil {
		scheme = "https"
		o.tlsConfig.NextProtos = append(o.tlsConfig.NextProtos, "h2")
		transport.TLSClientConfig = o.tlsConfig
	}

	return &ClientConn{
		target:     target,
		opts:       o,
		httpClient: &http.Client{Transport: transport, Timeout: 30 * time.Second},
		scheme:     scheme,
	}
}

// Invoke makes a unary RPC call.
func (cc *ClientConn) Invoke(ctx context.Context, method string, reqData []byte) ([]byte, error) {
	url := cc.scheme + "://" + cc.target + method

	var buf bytes.Buffer
	if err := writeMessage(&buf, reqData); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= cc.opts.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * 100 * time.Millisecond
			if backoff > 32*time.Second {
				backoff = 32 * time.Second
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		result, err := cc.doInvoke(ctx, url, buf.Bytes())
		if err == nil {
			return result, nil
		}
		lastErr = err

		// Only retry on connection errors, not gRPC status errors
		if _, ok := err.(*StatusError); ok {
			return nil, err
		}
	}
	return nil, lastErr
}

func (cc *ClientConn) doInvoke(ctx context.Context, url string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/grpc+proto")
	req.Header.Set("TE", "trailers")

	resp, err := cc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Check status from headers or trailers
	grpcStatus := resp.Header.Get("Grpc-Status")
	if grpcStatus == "" {
		grpcStatus = resp.Trailer.Get("Grpc-Status")
	}
	if grpcStatus != "" && grpcStatus != "0" {
		grpcMsg := resp.Header.Get("Grpc-Message")
		if grpcMsg == "" {
			grpcMsg = resp.Trailer.Get("Grpc-Message")
		}
		return nil, NewStatusError(parseInt(grpcStatus), grpcMsg)
	}

	if len(respBody) < 5 {
		return nil, fmt.Errorf("response too short: %d bytes", len(respBody))
	}

	// Skip the 5-byte gRPC frame header
	return respBody[5:], nil
}

// Close closes the client connection.
func (cc *ClientConn) Close() {
	cc.httpClient.CloseIdleConnections()
}

func parseInt(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}
