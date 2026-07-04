//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/asymmetric-effort/ginger/internal/health"
	"github.com/asymmetric-effort/ginger/internal/pipeline"
	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
	"github.com/asymmetric-effort/ginger/internal/query"
	"github.com/asymmetric-effort/ginger/internal/storage/memory"
)

func TestFullPipeline(t *testing.T) {
	backend := memory.NewBackend(10000)
	p := pipeline.New(pipeline.Config{QueueSize: 100})
	p.AddExporter(&memExporter{w: backend})
	p.Start(context.Background())
	defer p.Shutdown(context.Background())

	recv := otlp.NewHTTPReceiver(p, 0)
	querySvc := query.NewService(backend, backend, nil)
	httpHandler := query.NewHTTPHandler(querySvc)
	healthHandler := health.NewHandler()
	healthHandler.SetReady(true)

	mux := http.NewServeMux()
	mux.Handle("/v1/traces", recv)
	httpHandler.RegisterRoutes(mux)
	mux.HandleFunc("/health/live", healthHandler.LiveHandler())
	mux.HandleFunc("/health/ready", healthHandler.ReadyHandler())

	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	defer srv.Close()
	addr := ln.Addr().String()

	// Send 50 traces via protobuf (JSON omits TraceID/SpanID due to json:"-" tags)
	for i := byte(1); i <= 50; i++ {
		td := makeTD(i)
		body, _ := otlp.Marshal(td)
		resp, err := http.Post("http://"+addr+"/v1/traces", "application/x-protobuf", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}

	time.Sleep(2 * time.Second)

	// Query back
	resp, err := http.Get("http://" + addr + "/api/v3/services")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var services []string
	json.Unmarshal(body, &services)
	if len(services) == 0 {
		t.Error("no services after sending 50 traces")
	}

	// Health check
	resp, _ = http.Get("http://" + addr + "/health/ready")
	if resp.StatusCode != 200 {
		t.Error("should be ready")
	}
}

func makeTD(i byte) otlp.TracesData {
	resAttrs := otlp.NewAttributes()
	resAttrs.Set("service.name", otlp.StringValue("e2e-service"))
	return otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			Resource: otlp.Resource{Attributes: resAttrs},
			ScopeSpans: []otlp.ScopeSpans{{
				Spans: []otlp.Span{{
					TraceID: [16]byte{i}, SpanID: [8]byte{i}, Name: "op",
					StartTimeUnixNano: uint64(time.Now().UnixNano()),
					EndTimeUnixNano:   uint64(time.Now().Add(time.Millisecond).UnixNano()),
				}},
			}},
		}},
	}
}

type memExporter struct{ w *memory.Backend }

func (e *memExporter) ExportTraces(ctx context.Context, td otlp.TracesData) error {
	return e.w.WriteSpans(ctx, td)
}
func (e *memExporter) Shutdown(_ context.Context) error { return nil }
