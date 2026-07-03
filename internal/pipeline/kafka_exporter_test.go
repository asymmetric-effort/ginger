package pipeline

import (
	"context"
	"net"
	"testing"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

func TestKafkaExporterDefaults(t *testing.T) {
	ke := NewKafkaExporter(KafkaExporterConfig{})
	if ke.config.BatchSize != 100 {
		t.Errorf("default batch = %d", ke.config.BatchSize)
	}
	if ke.config.Topic != "ginger-spans" {
		t.Errorf("default topic = %q", ke.config.Topic)
	}
}

func TestKafkaExporterBatch(t *testing.T) {
	ke := NewKafkaExporter(KafkaExporterConfig{BatchSize: 5})

	td := otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			ScopeSpans: []otlp.ScopeSpans{{
				Spans: []otlp.Span{{TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "op",
					StartTimeUnixNano: 1, EndTimeUnixNano: 2}},
			}},
		}},
	}

	for i := 0; i < 3; i++ {
		ke.ExportTraces(context.Background(), td)
	}
	// Should not have flushed yet (3 < 5)
}

func TestKafkaExporterShutdown(t *testing.T) {
	ke := NewKafkaExporter(KafkaExporterConfig{})
	err := ke.Shutdown(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}

func TestKafkaReceiverDefaults(t *testing.T) {
	kr := NewKafkaReceiver(KafkaReceiverConfig{}, nil)
	if kr.config.Topic != "ginger-spans" {
		t.Errorf("default topic = %q", kr.config.Topic)
	}
	if kr.config.GroupID != "ginger-consumer" {
		t.Errorf("default group = %q", kr.config.GroupID)
	}
}

func TestKafkaReceiverStartShutdown(t *testing.T) {
	kr := NewKafkaReceiver(KafkaReceiverConfig{}, nil)
	ctx := context.Background()
	kr.Start(ctx, nil)
	kr.Shutdown(ctx)
}

func TestKafkaExporterFlushWithDataNoBroker(t *testing.T) {
	// Test flush with actual data but no broker configured → "no brokers configured"
	ke := NewKafkaExporter(KafkaExporterConfig{BatchSize: 1})

	td := otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			ScopeSpans: []otlp.ScopeSpans{{
				Spans: []otlp.Span{{TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "op",
					StartTimeUnixNano: 1, EndTimeUnixNano: 2}},
			}},
		}},
	}

	// ExportTraces with BatchSize=1 triggers flush immediately
	err := ke.ExportTraces(context.Background(), td)
	if err == nil {
		t.Error("expected error when no brokers configured")
	}
}

func TestKafkaExporterFlushWithDataBadBroker(t *testing.T) {
	// Test flush with bad broker address → "connect to broker" error
	ke := NewKafkaExporter(KafkaExporterConfig{
		BatchSize: 1,
		Brokers:   []string{"127.0.0.1:19999"}, // unlikely to have a listener
	})

	td := otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			ScopeSpans: []otlp.ScopeSpans{{
				Spans: []otlp.Span{{TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "op",
					StartTimeUnixNano: 1, EndTimeUnixNano: 2}},
			}},
		}},
	}

	err := ke.ExportTraces(context.Background(), td)
	if err == nil {
		t.Error("expected error when broker is unreachable")
	}
}

func TestKafkaExporterSendWithRealBroker(t *testing.T) {
	// Start a local TCP listener to simulate a broker
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().String()

	// Accept connection in background
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		conn.Close()
	}()

	ke := NewKafkaExporter(KafkaExporterConfig{
		BatchSize: 1,
		Brokers:   []string{addr},
	})

	td := otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			ScopeSpans: []otlp.ScopeSpans{{
				Spans: []otlp.Span{{TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "op",
					StartTimeUnixNano: 1, EndTimeUnixNano: 2}},
			}},
		}},
	}

	// This should connect to our listener and succeed
	err = ke.ExportTraces(context.Background(), td)
	if err != nil {
		t.Errorf("expected success with real broker: %v", err)
	}

	// Shutdown should close the connection
	err = ke.Shutdown(context.Background())
	if err != nil {
		t.Errorf("shutdown failed: %v", err)
	}
}

func TestKafkaExporterShutdownWithData(t *testing.T) {
	// Shutdown flushes remaining data; with no brokers, should return error
	ke := NewKafkaExporter(KafkaExporterConfig{BatchSize: 100})

	td := otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			ScopeSpans: []otlp.ScopeSpans{{
				Spans: []otlp.Span{{TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "op",
					StartTimeUnixNano: 1, EndTimeUnixNano: 2}},
			}},
		}},
	}
	// Add to batch without flushing (batch < BatchSize=100)
	ke.ExportTraces(context.Background(), td)

	// Shutdown should flush and fail because no brokers configured
	err := ke.Shutdown(context.Background())
	if err == nil {
		t.Error("expected error from Shutdown flush with no brokers")
	}
}
