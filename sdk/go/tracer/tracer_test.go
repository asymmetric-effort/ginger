package tracer

import (
	"context"
	"errors"
	"testing"
)

func TestTracerStartEnd(t *testing.T) {
	tr := New(Config{Service: "test"})
	ctx, span := tr.Start(context.Background(), "test-op")
	if ctx == nil || span == nil {
		t.Fatal("nil")
	}
	span.End()
	if !span.ended {
		t.Error("should be ended")
	}
	// Double end should not panic
	span.End()
}

func TestSpanAttributes(t *testing.T) {
	tr := New(Config{Service: "test"})
	_, span := tr.Start(context.Background(), "op")
	span.SetAttribute("key", "value")
	span.SetAttribute("count", 42)
	span.End()

	if span.attrs["key"] != "value" {
		t.Error("attr")
	}
}

func TestSpanRecordError(t *testing.T) {
	tr := New(Config{Service: "test"})
	_, span := tr.Start(context.Background(), "op")
	span.RecordError(errors.New("fail"))
	span.RecordError(nil) // should be ignored
	span.End()

	if len(span.events) != 1 {
		t.Errorf("events = %d", len(span.events))
	}
	if span.events[0].Attributes["exception.message"] != "fail" {
		t.Error("error message")
	}
}

func TestSpanAddEvent(t *testing.T) {
	tr := New(Config{Service: "test"})
	_, span := tr.Start(context.Background(), "op")
	span.AddEvent("request.start", map[string]string{"method": "GET"})
	span.End()

	if len(span.events) != 1 {
		t.Errorf("events = %d", len(span.events))
	}
}

func TestSpanSetStatus(t *testing.T) {
	tr := New(Config{Service: "test"})
	_, span := tr.Start(context.Background(), "op")
	span.SetStatus(StatusError, "something failed")
	span.End()

	if span.status != StatusError {
		t.Error("status")
	}
}

func TestWithSpanKind(t *testing.T) {
	tr := New(Config{Service: "test"})
	_, span := tr.Start(context.Background(), "op", WithSpanKind(SpanKindServer))
	if span.kind != SpanKindServer {
		t.Error("kind")
	}
	span.End()
}

func TestTracerDefaults(t *testing.T) {
	tr := New(Config{})
	if tr.config.SamplingRate != 1.0 {
		t.Error("default rate")
	}
	if tr.config.BatchSize != 512 {
		t.Error("default batch")
	}
}

func TestTracerShutdown(t *testing.T) {
	tr := New(Config{Service: "test"})
	tr.Shutdown() // should not panic
}
