package pipeline

import (
	"context"
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
