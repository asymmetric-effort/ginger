//go:build loadtest

package load

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/asymmetric-effort/ginger/internal/pipeline"
	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
	"github.com/asymmetric-effort/ginger/internal/storage/memory"
)

func TestSustainedLoad(t *testing.T) {
	backend := memory.NewBackend(100000)
	p := pipeline.New(pipeline.Config{QueueSize: 10000})
	p.AddExporter(&countExporter{w: backend})
	p.Start(context.Background())
	defer p.Shutdown(context.Background())

	var sent atomic.Int64
	var dropped atomic.Int64
	targetSpansPerSec := 10000
	duration := 10 * time.Second // shorter for CI

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	interval := time.Second / time.Duration(targetSpansPerSec)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			goto done
		case <-ticker.C:
			td := makeLoadTD(byte(sent.Load() % 256))
			if err := p.ConsumeTraces(context.Background(), td); err != nil {
				dropped.Add(1)
			} else {
				sent.Add(1)
			}
		}
	}
done:
	elapsed := time.Since(start)
	totalSent := sent.Load()
	totalDropped := dropped.Load()
	rate := float64(totalSent) / elapsed.Seconds()

	t.Logf("Load test: sent=%d dropped=%d rate=%.0f spans/s elapsed=%v",
		totalSent, totalDropped, rate, elapsed)

	dropRate := float64(totalDropped) / float64(totalSent+totalDropped)
	if dropRate > 0.01 {
		t.Errorf("drop rate %.2f%% exceeds 1%% threshold", dropRate*100)
	}
}

func makeLoadTD(i byte) otlp.TracesData {
	resAttrs := otlp.NewAttributes()
	resAttrs.Set("service.name", otlp.StringValue("load-test"))
	return otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			Resource: otlp.Resource{Attributes: resAttrs},
			ScopeSpans: []otlp.ScopeSpans{{
				Spans: []otlp.Span{{
					TraceID: [16]byte{i}, SpanID: [8]byte{i}, Name: "load-op",
					StartTimeUnixNano: uint64(time.Now().UnixNano()),
					EndTimeUnixNano:   uint64(time.Now().Add(time.Millisecond).UnixNano()),
				}},
			}},
		}},
	}
}

type countExporter struct{ w *memory.Backend }

func (e *countExporter) ExportTraces(ctx context.Context, td otlp.TracesData) error {
	return e.w.WriteSpans(ctx, td)
}
func (e *countExporter) Shutdown(_ context.Context) error { return nil }
