package integration

import (
	"bytes"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
	"github.com/asymmetric-effort/ginger/internal/protocol/zipkin"
)

func TestOTLPHTTPReceiverIntegration(t *testing.T) {
	h := NewHarness(t)

	recv := otlp.NewHTTPReceiver(h.Pipeline, 0)
	mux := http.NewServeMux()
	mux.Handle("/v1/traces", recv)

	ln := listenLocal(t)
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })

	td := makeTestTD([16]byte{0xAA}, "otlp-svc", "GET /data")
	body, _ := otlp.Marshal(td)

	resp, err := http.Post("http://"+ln.Addr().String()+"/v1/traces",
		"application/x-protobuf", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("OTLP status = %d", resp.StatusCode)
	}

	time.Sleep(200 * time.Millisecond)

	got, err := h.QueryTrace([16]byte{0xAA})
	if err != nil {
		t.Fatalf("trace not found: %v", err)
	}
	if len(got.ResourceSpans) == 0 {
		t.Error("no spans")
	}
}

func TestZipkinHTTPReceiverIntegration(t *testing.T) {
	h := NewHarness(t)

	zipkinRecv := zipkin.NewHTTPReceiver(h.Pipeline, 0)
	mux := http.NewServeMux()
	mux.Handle("/api/v2/spans", zipkinRecv)

	ln := listenLocal(t)
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })

	body := `[{"traceId":"bb000000000000000000000000000000","id":"0000000000000001","name":"zipkin-op","timestamp":1000000,"duration":500000,"localEndpoint":{"serviceName":"zipkin-svc"}}]`

	resp, err := http.Post("http://"+ln.Addr().String()+"/api/v2/spans",
		"application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 202 {
		t.Errorf("Zipkin status = %d", resp.StatusCode)
	}

	time.Sleep(200 * time.Millisecond)

	if !h.WaitForService("zipkin-svc", 2*time.Second) {
		t.Error("zipkin service not found")
	}
}

func listenLocal(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return ln
}
