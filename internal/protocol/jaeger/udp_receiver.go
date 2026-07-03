package jaeger

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
)

// UDPReceiver receives Jaeger spans via UDP (Thrift Compact or Binary).
type UDPReceiver struct {
	sink        TracesConsumer
	addr        string
	conn        *net.UDPConn
	bufSize     int
	compact     bool // true for Compact (6831), false for Binary (6832)
	received    atomic.Int64
	dropped     atomic.Int64
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	workerCount int
}

// NewUDPReceiver creates a UDP receiver.
// compact=true for Thrift Compact (port 6831), compact=false for Binary (6832).
func NewUDPReceiver(sink TracesConsumer, addr string, compact bool, workerCount int) *UDPReceiver {
	if workerCount <= 0 {
		workerCount = 4
	}
	return &UDPReceiver{
		sink:        sink,
		addr:        addr,
		bufSize:     65535,
		compact:     compact,
		workerCount: workerCount,
	}
}

// Start begins listening for UDP datagrams.
func (r *UDPReceiver) Start(ctx context.Context) error {
	udpAddr, err := net.ResolveUDPAddr("udp", r.addr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	r.conn = conn

	ctx, r.cancel = context.WithCancel(ctx)

	// Fixed worker pool reads from the single UDP socket
	for i := 0; i < r.workerCount; i++ {
		r.wg.Add(1)
		go r.readLoop(ctx)
	}

	return nil
}

// Shutdown stops the receiver.
func (r *UDPReceiver) Shutdown(_ context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}
	if r.conn != nil {
		r.conn.Close()
	}
	r.wg.Wait()
	return nil
}

// Received returns total received count.
func (r *UDPReceiver) Received() int64 { return r.received.Load() }

// Dropped returns total dropped count.
func (r *UDPReceiver) Dropped() int64 { return r.dropped.Load() }

// Addr returns the listener address.
func (r *UDPReceiver) Addr() string {
	if r.conn != nil {
		return r.conn.LocalAddr().String()
	}
	return r.addr
}

func (r *UDPReceiver) readLoop(ctx context.Context) {
	defer r.wg.Done()
	buf := make([]byte, r.bufSize)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, _, err := r.conn.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				r.dropped.Add(1)
				continue
			}
		}

		data := make([]byte, n)
		copy(data, buf[:n])

		batch, err := decodeThriftBatch(data)
		if err != nil {
			r.dropped.Add(1)
			continue
		}

		td := BatchToOTLP(batch)
		if err := r.sink.ConsumeTraces(ctx, td); err != nil {
			r.dropped.Add(1)
			continue
		}

		r.received.Add(int64(len(batch.Spans)))
	}
}
