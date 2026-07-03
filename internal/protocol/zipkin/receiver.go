package zipkin

import (
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"sync/atomic"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

// TracesConsumer receives trace data.
type TracesConsumer interface {
	ConsumeTraces(ctx context.Context, td otlp.TracesData) error
}

// HTTPReceiver receives Zipkin v2 JSON spans via POST /api/v2/spans.
type HTTPReceiver struct {
	sink          TracesConsumer
	spansReceived atomic.Int64
	maxBodyBytes  int64
}

// NewHTTPReceiver creates a new Zipkin HTTP receiver.
func NewHTTPReceiver(sink TracesConsumer, maxBodyBytes int64) *HTTPReceiver {
	if maxBodyBytes <= 0 {
		maxBodyBytes = 4 * 1024 * 1024
	}
	return &HTTPReceiver{sink: sink, maxBodyBytes: maxBodyBytes}
}

// ServeHTTP handles POST /api/v2/spans.
func (r *HTTPReceiver) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body := http.MaxBytesReader(w, req.Body, r.maxBodyBytes)
	defer body.Close()

	var reader io.Reader = body
	if req.Header.Get("Content-Encoding") == "gzip" {
		gr, err := gzip.NewReader(body)
		if err != nil {
			http.Error(w, "invalid gzip", http.StatusBadRequest)
			return
		}
		defer gr.Close()
		reader = gr
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}

	spans, err := UnmarshalArray(data)
	if err != nil {
		http.Error(w, "decode: "+err.Error(), http.StatusBadRequest)
		return
	}

	td := ToOTLP(spans)

	if err := r.sink.ConsumeTraces(req.Context(), td); err != nil {
		http.Error(w, err.Error(), http.StatusTooManyRequests)
		return
	}

	r.spansReceived.Add(int64(len(spans)))
	w.WriteHeader(http.StatusAccepted)
}

// SpansReceived returns the total spans received.
func (r *HTTPReceiver) SpansReceived() int64 {
	return r.spansReceived.Load()
}
