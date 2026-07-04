package pipeline

import (
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"sync"
	"sync/atomic"
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

// correlationID is a monotonically increasing counter for Kafka request correlation.
var correlationID atomic.Int32

func (ke *KafkaExporter) ensureConn() error {
	if ke.conn != nil {
		return nil
	}
	if len(ke.config.Brokers) == 0 {
		return fmt.Errorf("no brokers configured")
	}
	conn, err := net.DialTimeout("tcp", ke.config.Brokers[0], 5*time.Second)
	if err != nil {
		return fmt.Errorf("connect to broker: %w", err)
	}
	ke.conn = conn
	return nil
}

func (ke *KafkaExporter) send(data []byte) error {
	if err := ke.ensureConn(); err != nil {
		return err
	}

	msg := buildKafkaMessage(data)
	req := buildProduceRequest(ke.config.Topic, 0, msg, int32(correlationID.Add(1)))

	if _, err := ke.conn.Write(req); err != nil {
		// Connection failed; clear it so next call reconnects.
		ke.conn.Close()
		ke.conn = nil
		return fmt.Errorf("write produce request: %w", err)
	}
	return nil
}

// buildKafkaMessage builds a Kafka v0 message (magic=0):
//
//	8-byte offset | 4-byte message size | 4-byte CRC | 1-byte magic |
//	1-byte attributes | 4-byte key length (-1 null) | 4-byte value length | value
func buildKafkaMessage(value []byte) []byte {
	// Inner message (without offset and size prefix): CRC + magic + attr + key len + val len + val
	innerSize := 4 + 1 + 1 + 4 + 4 + len(value) // CRC(4) magic(1) attr(1) keyLen(4) valLen(4) value
	inner := make([]byte, innerSize)

	// magic byte = 0
	inner[4] = 0
	// attributes = 0 (no compression)
	inner[5] = 0
	// key length = -1 (null key)
	binary.BigEndian.PutUint32(inner[6:10], 0xFFFFFFFF)
	// value length
	binary.BigEndian.PutUint32(inner[10:14], uint32(len(value)))
	// value bytes
	copy(inner[14:], value)

	// CRC32 over bytes after the CRC field itself (magic + attr + key len + val len + val)
	crc := crc32.ChecksumIEEE(inner[4:])
	binary.BigEndian.PutUint32(inner[0:4], crc)

	// Full record: 8-byte offset + 4-byte message size + inner
	recordSize := 8 + 4 + innerSize
	record := make([]byte, recordSize)
	// offset = 0
	binary.BigEndian.PutUint64(record[0:8], 0)
	// message size = length of inner
	binary.BigEndian.PutUint32(record[8:12], uint32(innerSize))
	copy(record[12:], inner)
	return record
}

// buildProduceRequest frames a Kafka Produce API (key=0, version=0) request.
//
// Wire format:
//
//	4-byte total length (everything after this field)
//	2-byte API key (0 = Produce)
//	2-byte API version (0)
//	4-byte correlation ID
//	2-byte client ID length + client ID bytes
//	2-byte required acks (1 = leader ack)
//	4-byte timeout ms
//	4-byte topic array count (1)
//	  2-byte topic name length + topic bytes
//	  4-byte partition array count (1)
//	    4-byte partition index
//	    4-byte message set size
//	    message set bytes
func buildProduceRequest(topic string, partition int32, messageSet []byte, corrID int32) []byte {
	clientID := "ginger"
	topicBytes := []byte(topic)
	clientIDBytes := []byte(clientID)

	// Calculate body size (everything after the 4-byte length prefix)
	bodySize := 2 + 2 + 4 + // api key + version + correlation ID
		2 + len(clientIDBytes) + // client ID
		2 + 4 + // required acks + timeout
		4 + // topic array count
		2 + len(topicBytes) + // topic name
		4 + // partition array count
		4 + // partition index
		4 + // message set size
		len(messageSet) // message set

	buf := make([]byte, 4+bodySize)
	off := 0

	// Total request length (excludes this 4-byte field itself)
	binary.BigEndian.PutUint32(buf[off:], uint32(bodySize))
	off += 4

	// API key = 0 (Produce)
	binary.BigEndian.PutUint16(buf[off:], 0)
	off += 2

	// API version = 0
	binary.BigEndian.PutUint16(buf[off:], 0)
	off += 2

	// Correlation ID
	binary.BigEndian.PutUint32(buf[off:], uint32(corrID))
	off += 4

	// Client ID
	binary.BigEndian.PutUint16(buf[off:], uint16(len(clientIDBytes)))
	off += 2
	copy(buf[off:], clientIDBytes)
	off += len(clientIDBytes)

	// Required acks = 1
	binary.BigEndian.PutUint16(buf[off:], 1)
	off += 2

	// Timeout ms = 5000
	binary.BigEndian.PutUint32(buf[off:], 5000)
	off += 4

	// Topic array count = 1
	binary.BigEndian.PutUint32(buf[off:], 1)
	off += 4

	// Topic name
	binary.BigEndian.PutUint16(buf[off:], uint16(len(topicBytes)))
	off += 2
	copy(buf[off:], topicBytes)
	off += len(topicBytes)

	// Partition array count = 1
	binary.BigEndian.PutUint32(buf[off:], 1)
	off += 4

	// Partition index
	binary.BigEndian.PutUint32(buf[off:], uint32(partition))
	off += 4

	// Message set size
	binary.BigEndian.PutUint32(buf[off:], uint32(len(messageSet)))
	off += 4

	// Message set
	copy(buf[off:], messageSet)

	return buf
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

	if len(kr.config.Brokers) == 0 {
		<-ctx.Done()
		return
	}

	var conn net.Conn
	var fetchOffset int64

	for {
		select {
		case <-ctx.Done():
			if conn != nil {
				conn.Close()
			}
			return
		default:
		}

		// Ensure connection
		if conn == nil {
			var err error
			conn, err = net.DialTimeout("tcp", kr.config.Brokers[0], 5*time.Second)
			if err != nil {
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Second):
					continue
				}
			}
		}

		// Send Fetch request and parse response
		req := buildFetchRequest(kr.config.Topic, 0, fetchOffset, int32(correlationID.Add(1)))
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err := conn.Write(req); err != nil {
			conn.Close()
			conn = nil
			continue
		}

		conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		messages, nextOffset, err := parseFetchResponse(conn)
		if err != nil {
			conn.Close()
			conn = nil
			continue
		}

		if nextOffset > fetchOffset {
			fetchOffset = nextOffset
		}

		for _, msgVal := range messages {
			td, err := otlp.Unmarshal(msgVal)
			if err != nil {
				continue
			}
			kr.sink.ConsumeTraces(ctx, td)
		}
	}
}

// buildFetchRequest creates a Kafka Fetch API (key=1, version=0) request.
//
// Wire format:
//
//	4-byte total length
//	2-byte API key (1 = Fetch)
//	2-byte API version (0)
//	4-byte correlation ID
//	2-byte client ID length + client ID
//	4-byte replica ID (-1 for consumer)
//	4-byte max wait ms
//	4-byte min bytes
//	4-byte topic array count (1)
//	  2-byte topic name length + topic
//	  4-byte partition array count (1)
//	    4-byte partition
//	    8-byte fetch offset
//	    4-byte max bytes
func buildFetchRequest(topic string, partition int32, offset int64, corrID int32) []byte {
	clientID := "ginger"
	topicBytes := []byte(topic)
	clientIDBytes := []byte(clientID)

	bodySize := 2 + 2 + 4 + // api key + version + correlation ID
		2 + len(clientIDBytes) + // client ID
		4 + 4 + 4 + // replica ID + max wait + min bytes
		4 + // topic array count
		2 + len(topicBytes) + // topic name
		4 + // partition array count
		4 + 8 + 4 // partition + offset + max bytes

	buf := make([]byte, 4+bodySize)
	off := 0

	binary.BigEndian.PutUint32(buf[off:], uint32(bodySize))
	off += 4

	// API key = 1 (Fetch)
	binary.BigEndian.PutUint16(buf[off:], 1)
	off += 2

	// API version = 0
	binary.BigEndian.PutUint16(buf[off:], 0)
	off += 2

	// Correlation ID
	binary.BigEndian.PutUint32(buf[off:], uint32(corrID))
	off += 4

	// Client ID
	binary.BigEndian.PutUint16(buf[off:], uint16(len(clientIDBytes)))
	off += 2
	copy(buf[off:], clientIDBytes)
	off += len(clientIDBytes)

	// Replica ID = -1 (consumer)
	binary.BigEndian.PutUint32(buf[off:], 0xFFFFFFFF)
	off += 4

	// Max wait ms = 5000
	binary.BigEndian.PutUint32(buf[off:], 5000)
	off += 4

	// Min bytes = 1
	binary.BigEndian.PutUint32(buf[off:], 1)
	off += 4

	// Topic array count = 1
	binary.BigEndian.PutUint32(buf[off:], 1)
	off += 4

	// Topic name
	binary.BigEndian.PutUint16(buf[off:], uint16(len(topicBytes)))
	off += 2
	copy(buf[off:], topicBytes)
	off += len(topicBytes)

	// Partition array count = 1
	binary.BigEndian.PutUint32(buf[off:], 1)
	off += 4

	// Partition
	binary.BigEndian.PutUint32(buf[off:], uint32(partition))
	off += 4

	// Fetch offset
	binary.BigEndian.PutUint64(buf[off:], uint64(offset))
	off += 8

	// Max bytes = 1MB
	binary.BigEndian.PutUint32(buf[off:], 1024*1024)

	return buf
}

// parseFetchResponse reads a Kafka Fetch response from conn and extracts message values.
// Returns the extracted message values and the next offset to fetch from.
func parseFetchResponse(r io.Reader) ([][]byte, int64, error) {
	// Read 4-byte response length
	var respLen uint32
	if err := binary.Read(r, binary.BigEndian, &respLen); err != nil {
		return nil, 0, fmt.Errorf("read response length: %w", err)
	}
	if respLen > 16*1024*1024 {
		return nil, 0, fmt.Errorf("response too large: %d", respLen)
	}

	respBuf := make([]byte, respLen)
	if _, err := io.ReadFull(r, respBuf); err != nil {
		return nil, 0, fmt.Errorf("read response body: %w", err)
	}

	// Minimum fetch response: correlation(4) + topic count(4) = 8
	if len(respBuf) < 8 {
		return nil, 0, fmt.Errorf("response too short")
	}

	off := 0
	// Skip correlation ID
	off += 4

	// Topic array count
	topicCount := int(binary.BigEndian.Uint32(respBuf[off:]))
	off += 4

	var messages [][]byte
	var nextOffset int64

	for t := 0; t < topicCount && off < len(respBuf); t++ {
		// Topic name
		if off+2 > len(respBuf) {
			break
		}
		topicNameLen := int(binary.BigEndian.Uint16(respBuf[off:]))
		off += 2
		off += topicNameLen // skip topic name

		// Partition count
		if off+4 > len(respBuf) {
			break
		}
		partCount := int(binary.BigEndian.Uint32(respBuf[off:]))
		off += 4

		for p := 0; p < partCount && off < len(respBuf); p++ {
			// partition(4) + error(2) + highWaterMark(8) + messageSetSize(4)
			if off+18 > len(respBuf) {
				break
			}
			off += 4 // partition
			off += 2 // error code
			off += 8 // high watermark

			msgSetSize := int(binary.BigEndian.Uint32(respBuf[off:]))
			off += 4

			msgSetEnd := off + msgSetSize
			if msgSetEnd > len(respBuf) {
				msgSetEnd = len(respBuf)
			}

			// Parse message set
			for off+12 < msgSetEnd {
				// 8-byte offset + 4-byte message size
				msgOffset := int64(binary.BigEndian.Uint64(respBuf[off:]))
				off += 8
				msgSize := int(binary.BigEndian.Uint32(respBuf[off:]))
				off += 4

				if off+msgSize > msgSetEnd || msgSize < 14 {
					break
				}

				// Parse message: CRC(4) + magic(1) + attr(1) + keyLen(4) + valLen(4) + val
				msgStart := off
				off += 4 // CRC
				off += 1 // magic
				off += 1 // attributes

				keyLen := int(int32(binary.BigEndian.Uint32(respBuf[off:])))
				off += 4
				if keyLen > 0 {
					off += keyLen
				}

				if off+4 > msgStart+msgSize {
					off = msgStart + msgSize
					continue
				}
				valLen := int(int32(binary.BigEndian.Uint32(respBuf[off:])))
				off += 4

				if valLen > 0 && off+valLen <= msgStart+msgSize {
					val := make([]byte, valLen)
					copy(val, respBuf[off:off+valLen])
					messages = append(messages, val)
				}
				off = msgStart + msgSize

				if msgOffset+1 > nextOffset {
					nextOffset = msgOffset + 1
				}
			}
			off = msgSetEnd
		}
	}

	return messages, nextOffset, nil
}
