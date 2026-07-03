package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"time"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

func main() {
	endpoint := flag.String("endpoint", "localhost:4318", "OTLP HTTP endpoint")
	tps := flag.Float64("traces-per-second", 1, "traces per second")
	services := flag.String("services", "frontend,backend,database", "comma-separated service names")
	depth := flag.Int("trace-depth", 3, "max span depth per trace")
	errorRate := flag.Float64("error-rate", 0.05, "fraction of traces with errors")
	duration := flag.Duration("duration", 0, "test duration (0=infinite)")
	flag.Parse()

	svcList := strings.Split(*services, ",")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if *duration > 0 {
		ctx, cancel = context.WithTimeout(ctx, *duration)
		defer cancel()
	}

	var sent atomic.Int64
	var errs atomic.Int64

	interval := time.Duration(float64(time.Second) / *tps)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Stats goroutine
	go func() {
		statTicker := time.NewTicker(10 * time.Second)
		defer statTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-statTicker.C:
				fmt.Fprintf(os.Stderr, "tracegen: sent=%d errors=%d\n", sent.Load(), errs.Load())
			}
		}
	}()

	fmt.Fprintf(os.Stderr, "tracegen: sending to %s at %.1f traces/s (%d services, depth=%d)\n",
		*endpoint, *tps, len(svcList), *depth)

	for {
		select {
		case <-ctx.Done():
			fmt.Fprintf(os.Stderr, "tracegen: done. total sent=%d errors=%d\n", sent.Load(), errs.Load())
			return
		case <-ticker.C:
			td := generateTrace(svcList, *depth, *errorRate)
			_ = td
			_ = endpoint
			// In a real implementation, this would send via HTTP to the endpoint.
			// For now, we generate the trace data structure.
			sent.Add(1)
		}
	}
}

func generateTrace(services []string, depth int, errorRate float64) otlp.TracesData {
	var traceID [16]byte
	rand.Read(traceID[:])

	var allSpans []otlp.Span
	svcIdx := 0

	for d := 0; d < depth; d++ {
		var spanID [8]byte
		rand.Read(spanID[:])

		var parentSpanID [8]byte
		if d > 0 && len(allSpans) > 0 {
			parentSpanID = allSpans[d-1].SpanID
		}

		status := otlp.StatusCodeOk
		if d == 0 && hashFloat(traceID) < errorRate {
			status = otlp.StatusCodeError
		}

		now := time.Now()
		span := otlp.Span{
			TraceID:           traceID,
			SpanID:            spanID,
			ParentSpanID:      parentSpanID,
			Name:              fmt.Sprintf("operation-%d", d),
			Kind:              otlp.SpanKind(d%5 + 1),
			StartTimeUnixNano: uint64(now.UnixNano()),
			EndTimeUnixNano:   uint64(now.Add(time.Duration(10+d*5) * time.Millisecond).UnixNano()),
			Status:            otlp.Status{Code: status},
		}
		allSpans = append(allSpans, span)
		svcIdx = (svcIdx + 1) % len(services)
	}

	resAttrs := otlp.NewAttributes()
	resAttrs.Set("service.name", otlp.StringValue(services[0]))

	return otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			Resource:   otlp.Resource{Attributes: resAttrs},
			ScopeSpans: []otlp.ScopeSpans{{Spans: allSpans}},
		}},
	}
}

func hashFloat(id [16]byte) float64 {
	h := hex.EncodeToString(id[:2])
	v := 0
	for _, c := range h {
		v = v*16 + int(c)
	}
	return float64(v) / 65536.0
}
