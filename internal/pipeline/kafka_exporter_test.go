package pipeline

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"testing"
	"time"

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

func TestKafkaProducerSendMockTCP(t *testing.T) {
	// Start a mock TCP server that reads Kafka Produce requests.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	var received []byte
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Read the 4-byte request length prefix
		lenBuf := make([]byte, 4)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return
		}
		reqLen := binary.BigEndian.Uint32(lenBuf)

		// Read the full request body
		body := make([]byte, reqLen)
		if _, err := io.ReadFull(conn, body); err != nil {
			return
		}
		received = append(lenBuf, body...)
	}()

	ke := NewKafkaExporter(KafkaExporterConfig{
		BatchSize: 1,
		Brokers:   []string{ln.Addr().String()},
		Topic:     "test-topic",
	})

	td := otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			ScopeSpans: []otlp.ScopeSpans{{
				Spans: []otlp.Span{{TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "test-op",
					StartTimeUnixNano: 1, EndTimeUnixNano: 2}},
			}},
		}},
	}

	err = ke.ExportTraces(context.Background(), td)
	if err != nil {
		t.Fatalf("ExportTraces: %v", err)
	}
	ke.Shutdown(context.Background())

	<-done

	if len(received) == 0 {
		t.Fatal("expected to receive data on mock TCP server")
	}

	// Verify the request is a Produce request (API key = 0)
	if len(received) < 8 {
		t.Fatalf("received data too short: %d bytes", len(received))
	}
	apiKey := binary.BigEndian.Uint16(received[4:6])
	if apiKey != 0 {
		t.Errorf("API key = %d, want 0 (Produce)", apiKey)
	}
	apiVersion := binary.BigEndian.Uint16(received[6:8])
	if apiVersion != 0 {
		t.Errorf("API version = %d, want 0", apiVersion)
	}
}

func TestKafkaConsumerFetchMockTCP(t *testing.T) {
	// Create a simple OTLP trace to use as the message value
	td := otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			ScopeSpans: []otlp.ScopeSpans{{
				Spans: []otlp.Span{{TraceID: [16]byte{2}, SpanID: [8]byte{2}, Name: "fetched-op",
					StartTimeUnixNano: 10, EndTimeUnixNano: 20}},
			}},
		}},
	}
	msgData, err := otlp.Marshal(td)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Build a mock Kafka Fetch response containing this message
	fetchResp := buildMockFetchResponse(1, "ginger-spans", 0, msgData)

	// Start mock broker
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Read the Fetch request (just drain it)
		lenBuf := make([]byte, 4)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return
		}
		reqLen := binary.BigEndian.Uint32(lenBuf)
		discard := make([]byte, reqLen)
		if _, err := io.ReadFull(conn, discard); err != nil {
			return
		}

		// Write the mock Fetch response
		conn.Write(fetchResp)

		// Keep connection open briefly then close (consumer will see EOF and retry)
		select {}
	}()

	// Create a sink to capture consumed traces
	sink := &mockSink{}
	kr := NewKafkaReceiver(KafkaReceiverConfig{
		Brokers: []string{ln.Addr().String()},
		Topic:   "ginger-spans",
	}, sink)

	ctx, cancel := context.WithCancel(context.Background())
	kr.Start(ctx, nil)

	// Wait for the consumer to process
	deadline := time.After(3 * time.Second)
	for {
		select {
		case <-deadline:
			cancel()
			kr.Shutdown(context.Background())
			t.Fatal("timed out waiting for consumer to receive trace")
			return
		default:
		}
		sink.mu.Lock()
		got := len(sink.traces)
		sink.mu.Unlock()
		if got > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	cancel()
	kr.Shutdown(context.Background())

	sink.mu.Lock()
	defer sink.mu.Unlock()
	if len(sink.traces) == 0 {
		t.Fatal("expected at least one trace consumed")
	}
	if len(sink.traces[0].ResourceSpans) == 0 ||
		len(sink.traces[0].ResourceSpans[0].ScopeSpans) == 0 ||
		len(sink.traces[0].ResourceSpans[0].ScopeSpans[0].Spans) == 0 {
		t.Fatal("consumed trace has no spans")
	}
	span := sink.traces[0].ResourceSpans[0].ScopeSpans[0].Spans[0]
	if span.Name != "fetched-op" {
		t.Errorf("span name = %q, want fetched-op", span.Name)
	}
}

type mockSink struct {
	mu     sync.Mutex
	traces []otlp.TracesData
}

func (s *mockSink) ConsumeTraces(_ context.Context, td otlp.TracesData) error {
	s.mu.Lock()
	s.traces = append(s.traces, td)
	s.mu.Unlock()
	return nil
}

// buildMockFetchResponse creates a Kafka Fetch v0 response with a single message.
func buildMockFetchResponse(corrID int32, topic string, partition int32, value []byte) []byte {
	// Build the message
	msg := buildKafkaMessage(value)

	topicBytes := []byte(topic)
	// Response body: correlationID(4) + topicCount(4) +
	//   topicNameLen(2) + topicName + partCount(4) +
	//     partition(4) + errorCode(2) + highWatermark(8) + msgSetSize(4) + msgSet
	bodySize := 4 + 4 +
		2 + len(topicBytes) + 4 +
		4 + 2 + 8 + 4 + len(msg)

	buf := make([]byte, 4+bodySize)
	off := 0

	// Response length
	binary.BigEndian.PutUint32(buf[off:], uint32(bodySize))
	off += 4

	// Correlation ID
	binary.BigEndian.PutUint32(buf[off:], uint32(corrID))
	off += 4

	// Topic count = 1
	binary.BigEndian.PutUint32(buf[off:], 1)
	off += 4

	// Topic name
	binary.BigEndian.PutUint16(buf[off:], uint16(len(topicBytes)))
	off += 2
	copy(buf[off:], topicBytes)
	off += len(topicBytes)

	// Partition count = 1
	binary.BigEndian.PutUint32(buf[off:], 1)
	off += 4

	// Partition
	binary.BigEndian.PutUint32(buf[off:], uint32(partition))
	off += 4

	// Error code = 0
	binary.BigEndian.PutUint16(buf[off:], 0)
	off += 2

	// High watermark
	binary.BigEndian.PutUint64(buf[off:], 1)
	off += 8

	// Message set size
	binary.BigEndian.PutUint32(buf[off:], uint32(len(msg)))
	off += 4

	// Message set
	copy(buf[off:], msg)

	return buf
}

func TestBuildKafkaMessage(t *testing.T) {
	value := []byte("hello")
	msg := buildKafkaMessage(value)

	// offset(8) + msgSize(4) + CRC(4) + magic(1) + attr(1) + keyLen(4) + valLen(4) + value(5)
	expectedLen := 8 + 4 + 4 + 1 + 1 + 4 + 4 + 5
	if len(msg) != expectedLen {
		t.Fatalf("message length = %d, want %d", len(msg), expectedLen)
	}

	// Check offset = 0
	offset := binary.BigEndian.Uint64(msg[0:8])
	if offset != 0 {
		t.Errorf("offset = %d, want 0", offset)
	}

	// Check value length
	valLen := binary.BigEndian.Uint32(msg[22:26])
	if valLen != 5 {
		t.Errorf("value length = %d, want 5", valLen)
	}

	// Check value
	if string(msg[26:31]) != "hello" {
		t.Errorf("value = %q, want hello", string(msg[26:31]))
	}
}

func TestBuildProduceRequest(t *testing.T) {
	msg := buildKafkaMessage([]byte("test"))
	req := buildProduceRequest("my-topic", 0, msg, 42)

	if len(req) < 12 {
		t.Fatalf("request too short: %d", len(req))
	}

	// Check length prefix
	totalLen := binary.BigEndian.Uint32(req[0:4])
	if int(totalLen) != len(req)-4 {
		t.Errorf("length prefix = %d, want %d", totalLen, len(req)-4)
	}

	// API key = 0
	apiKey := binary.BigEndian.Uint16(req[4:6])
	if apiKey != 0 {
		t.Errorf("API key = %d, want 0", apiKey)
	}

	// API version = 0
	apiVer := binary.BigEndian.Uint16(req[6:8])
	if apiVer != 0 {
		t.Errorf("API version = %d, want 0", apiVer)
	}

	// Correlation ID = 42
	corrID := binary.BigEndian.Uint32(req[8:12])
	if corrID != 42 {
		t.Errorf("correlation ID = %d, want 42", corrID)
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
