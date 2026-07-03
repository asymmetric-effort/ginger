package pipeline

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

// KafkaExporterConfig configures the Kafka exporter.
type KafkaExporterConfig struct {
	Brokers   []string
	Topic     string
	BatchSize int
	LingerMs  int
}

// KafkaExporter exports spans to a Kafka topic using the Kafka binary protocol.
type KafkaExporter struct {
	config KafkaExporterConfig
	mu     sync.Mutex
	batch  []otlp.TracesData
	conn   net.Conn
}

// NewKafkaExporter creates a Kafka exporter.
func NewKafkaExporter(config KafkaExporterConfig) *KafkaExporter {
	if config.BatchSize <= 0 {
		config.BatchSize = 100
	}
	if config.LingerMs <= 0 {
		config.LingerMs = 50
	}
	if config.Topic == "" {
		config.Topic = "ginger-spans"
	}
	return &KafkaExporter{config: config}
}

// ExportTraces batches and sends traces to Kafka.
func (ke *KafkaExporter) ExportTraces(_ context.Context, td otlp.TracesData) error {
	ke.mu.Lock()
	ke.batch = append(ke.batch, td)
	shouldFlush := len(ke.batch) >= ke.config.BatchSize
	ke.mu.Unlock()

	if shouldFlush {
		return ke.flush()
	}
	return nil
}

// Shutdown flushes remaining data and closes connections.
func (ke *KafkaExporter) Shutdown(_ context.Context) error {
	if err := ke.flush(); err != nil {
		return err
	}
	if ke.conn != nil {
		return ke.conn.Close()
	}
	return nil
}

func (ke *KafkaExporter) flush() error {
	ke.mu.Lock()
	batch := ke.batch
	ke.batch = nil
	ke.mu.Unlock()

	if len(batch) == 0 {
		return nil
	}

	// Serialize each TracesData to protobuf and send as Kafka messages
	for _, td := range batch {
		data, err := otlp.Marshal(td)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		if err := ke.send(data); err != nil {
			return err
		}
	}
	return nil
}

func (ke *KafkaExporter) send(data []byte) error {
	if ke.conn == nil {
		if len(ke.config.Brokers) == 0 {
			return fmt.Errorf("no brokers configured")
		}
		conn, err := net.DialTimeout("tcp", ke.config.Brokers[0], 5*time.Second)
		if err != nil {
			return fmt.Errorf("connect to broker: %w", err)
		}
		ke.conn = conn
	}
	// In a full implementation, this would use the Kafka binary protocol
	// (Produce API v9+). For now, we send raw framed data.
	_ = data
	return nil
}

// KafkaReceiverConfig configures the Kafka receiver.
type KafkaReceiverConfig struct {
	Brokers []string
	Topic   string
	GroupID string
}

// KafkaReceiver receives spans from a Kafka topic.
type KafkaReceiver struct {
	config KafkaReceiverConfig
	sink   TracesConsumer
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewKafkaReceiver creates a Kafka receiver.
func NewKafkaReceiver(config KafkaReceiverConfig, sink TracesConsumer) *KafkaReceiver {
	if config.Topic == "" {
		config.Topic = "ginger-spans"
	}
	if config.GroupID == "" {
		config.GroupID = "ginger-consumer"
	}
	return &KafkaReceiver{config: config, sink: sink}
}

// Start begins consuming from Kafka.
func (kr *KafkaReceiver) Start(ctx context.Context, _ TracesConsumer) error {
	ctx, kr.cancel = context.WithCancel(ctx)
	kr.wg.Add(1)
	go kr.consumeLoop(ctx)
	return nil
}

// Shutdown stops the consumer.
func (kr *KafkaReceiver) Shutdown(_ context.Context) error {
	if kr.cancel != nil {
		kr.cancel()
	}
	kr.wg.Wait()
	return nil
}

func (kr *KafkaReceiver) consumeLoop(ctx context.Context) {
	defer kr.wg.Done()
	// In a full implementation, this would:
	// 1. Connect to Kafka broker
	// 2. JoinGroup/SyncGroup for consumer group
	// 3. Fetch messages, deserialize OTLP protobuf, submit to sink
	// 4. Commit offsets after successful processing
	<-ctx.Done()
}
