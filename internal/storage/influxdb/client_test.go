package influxdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/asymmetric-effort/ginger/internal/codec/influxlp"
)

func TestWriteSuccess(t *testing.T) {
	var received atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/api/v2/write") {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Token test-token" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{
		Endpoint: srv.URL,
		Org:      "myorg",
		Bucket:   "mybucket",
		Token:    "test-token",
	})
	defer c.Close()

	points := []influxlp.Point{{
		Measurement: "cpu",
		Tags:        []influxlp.Tag{{Key: "host", Value: "a"}},
		Fields:      []influxlp.Field{influxlp.FloatField("value", 0.64)},
		Timestamp:   time.Unix(0, 1000000000),
	}}

	err := c.Write(context.Background(), points)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if received.Load() != 1 {
		t.Errorf("received = %d", received.Load())
	}
}

func TestWriteNoToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("should not send Authorization when token is empty")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{Endpoint: srv.URL, Org: "o", Bucket: "b"})
	defer c.Close()

	err := c.Write(context.Background(), []influxlp.Point{{
		Measurement: "m", Fields: []influxlp.Field{influxlp.IntField("v", 1)},
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWriteClientError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{Endpoint: srv.URL, Org: "o", Bucket: "b"})
	defer c.Close()

	err := c.Write(context.Background(), []influxlp.Point{{
		Measurement: "m", Fields: []influxlp.Field{influxlp.IntField("v", 1)},
	}})
	if err == nil {
		t.Error("expected error for 400")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error = %v", err)
	}
}

func TestWriteRetryOn5xx(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := attempts.Add(1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{Endpoint: srv.URL, Org: "o", Bucket: "b"})
	defer c.Close()

	err := c.Write(context.Background(), []influxlp.Point{{
		Measurement: "m", Fields: []influxlp.Field{influxlp.IntField("v", 1)},
	}})
	if err != nil {
		t.Fatalf("should succeed after retry: %v", err)
	}
	if attempts.Load() != 3 {
		t.Errorf("attempts = %d, want 3", attempts.Load())
	}
}

func TestWriteRetryExhausted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{Endpoint: srv.URL, Org: "o", Bucket: "b"})
	defer c.Close()

	err := c.Write(context.Background(), []influxlp.Point{{
		Measurement: "m", Fields: []influxlp.Field{influxlp.IntField("v", 1)},
	}})
	if err == nil {
		t.Error("should fail after 3 retries")
	}
}

func TestWriteCircuitBreaker(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{Endpoint: srv.URL, Org: "o", Bucket: "b"})
	defer c.Close()

	// Exhaust retries twice to hit 6 failures > 5 threshold
	point := influxlp.Point{Measurement: "m", Fields: []influxlp.Field{influxlp.IntField("v", 1)}}
	c.Write(context.Background(), []influxlp.Point{point})
	c.Write(context.Background(), []influxlp.Point{point})

	// Circuit should be open now
	err := c.Write(context.Background(), []influxlp.Point{point})
	if err != ErrCircuitOpen {
		t.Errorf("expected circuit open, got %v", err)
	}
}

func TestWriteContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Second)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{Endpoint: srv.URL, Org: "o", Bucket: "b", WriteTimeout: 50 * time.Millisecond})
	defer c.Close()

	err := c.Write(context.Background(), []influxlp.Point{{
		Measurement: "m", Fields: []influxlp.Field{influxlp.IntField("v", 1)},
	}})
	if err == nil {
		t.Error("should timeout")
	}
}

func TestQuerySuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Token tk" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "text/csv")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("#group,false,false\n_time,_value,host\n2026-01-01T00:00:00Z,42,server01\n"))
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{Endpoint: srv.URL, Org: "o", Bucket: "b", Token: "tk"})
	defer c.Close()

	records, err := c.Query(context.Background(), `from(bucket: "b") |> range(start: -1h)`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %d", len(records))
	}
	if records[0].Values["_value"] != "42" {
		t.Errorf("value = %q", records[0].Values["_value"])
	}
}

func TestQueryNoToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("should not send auth")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("col\nval\n"))
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{Endpoint: srv.URL, Org: "o", Bucket: "b"})
	defer c.Close()

	_, err := c.Query(context.Background(), "query")
	if err != nil {
		t.Fatal(err)
	}
}

func TestQueryError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad query"))
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{Endpoint: srv.URL, Org: "o", Bucket: "b"})
	defer c.Close()

	_, err := c.Query(context.Background(), "bad")
	if err == nil {
		t.Error("expected error")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error = %v", err)
	}
}

func TestQueryEmptyResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(""))
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{Endpoint: srv.URL, Org: "o", Bucket: "b"})
	defer c.Close()

	records, err := c.Query(context.Background(), "q")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestParseCSV(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLen int
	}{
		{"empty", "", 0},
		{"header only", "a,b,c", 0},
		{"with data", "a,b\n1,2\n3,4", 2},
		{"with annotations", "#group,false\na,b\n1,2", 1},
		{"trailing newline", "a,b\n1,2\n", 1},
		{"all annotations", "#a\n#b", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			records, err := parseCSV(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if len(records) != tt.wantLen {
				t.Errorf("records = %d, want %d", len(records), tt.wantLen)
			}
		})
	}
}

func TestEscapeJSON(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`hello`, `hello`},
		{`say "hi"`, `say \"hi\"`},
		{"line\nbreak", `line\nbreak`},
		{`back\slash`, `back\\slash`},
	}
	for _, tt := range tests {
		got := escapeJSON(tt.input)
		if got != tt.want {
			t.Errorf("escapeJSON(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCircuitBreakerReset(t *testing.T) {
	c := NewClient(ClientConfig{Endpoint: "http://localhost:1", Org: "o", Bucket: "b"})
	defer c.Close()

	// Force circuit open
	c.circuitOpen.Store(true)
	c.circuitMu.Lock()
	c.circuitReset = time.Now().Add(-time.Second) // already past
	c.circuitMu.Unlock()

	if c.isCircuitOpen() {
		t.Error("circuit should have reset (past reset time)")
	}
}

func TestRecordSuccess(t *testing.T) {
	c := NewClient(ClientConfig{Endpoint: "http://localhost:1", Org: "o", Bucket: "b"})
	c.failures.Store(3)
	c.circuitOpen.Store(true)
	c.recordSuccess()
	if c.failures.Load() != 0 {
		t.Error("failures should reset")
	}
	if c.circuitOpen.Load() {
		t.Error("circuit should close")
	}
}

func TestClientDefaults(t *testing.T) {
	c := NewClient(ClientConfig{Endpoint: "http://localhost:1"})
	if c.config.WriteTimeout != 10*time.Second {
		t.Errorf("default write timeout = %v", c.config.WriteTimeout)
	}
	if c.config.QueryTimeout != 30*time.Second {
		t.Errorf("default query timeout = %v", c.config.QueryTimeout)
	}
	if c.config.MaxBatchSize != 5000 {
		t.Errorf("default batch size = %d", c.config.MaxBatchSize)
	}
}

func TestWriteContextCancelledDuringRetry(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{Endpoint: srv.URL, Org: "o", Bucket: "b"})
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := c.Write(ctx, []influxlp.Point{{
		Measurement: "m", Fields: []influxlp.Field{influxlp.IntField("v", 1)},
	}})
	if err == nil {
		t.Error("should fail with cancelled context")
	}
}

func TestWriteEncodeError(t *testing.T) {
	c := NewClient(ClientConfig{Endpoint: "http://localhost:1", Org: "o", Bucket: "b"})
	defer c.Close()

	// Empty measurement should cause encode error
	err := c.Write(context.Background(), []influxlp.Point{{
		Measurement: "", Fields: []influxlp.Field{influxlp.IntField("v", 1)},
	}})
	if err == nil {
		t.Error("expected encode error")
	}
}

func TestQueryConnectionError(t *testing.T) {
	c := NewClient(ClientConfig{Endpoint: "http://localhost:1", Org: "o", Bucket: "b", QueryTimeout: 100 * time.Millisecond})
	defer c.Close()

	_, err := c.Query(context.Background(), "q")
	if err == nil {
		t.Error("expected connection error")
	}
}

func TestWriteRetryOn429(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := attempts.Add(1)
		if count < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{Endpoint: srv.URL, Org: "o", Bucket: "b"})
	defer c.Close()

	err := c.Write(context.Background(), []influxlp.Point{{
		Measurement: "m", Fields: []influxlp.Field{influxlp.IntField("v", 1)},
	}})
	if err != nil {
		t.Fatalf("should retry 429: %v", err)
	}
}
