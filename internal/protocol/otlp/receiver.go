package otlp

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
)

// TracesConsumer receives trace data.
type TracesConsumer interface {
	ConsumeTraces(ctx context.Context, td TracesData) error
}

// HTTPReceiver receives OTLP traces via HTTP POST /v1/traces.
type HTTPReceiver struct {
	sink          TracesConsumer
	spansReceived atomic.Int64
	spansRejected atomic.Int64
	maxBodyBytes  int64
}

// NewHTTPReceiver creates a new OTLP HTTP receiver.
func NewHTTPReceiver(sink TracesConsumer, maxBodyBytes int64) *HTTPReceiver {
	if maxBodyBytes <= 0 {
		maxBodyBytes = 4 * 1024 * 1024
	}
	return &HTTPReceiver{sink: sink, maxBodyBytes: maxBodyBytes}
}

// ServeHTTP handles POST /v1/traces.
func (r *HTTPReceiver) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ct := req.Header.Get("Content-Type")

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

	var td TracesData
	switch ct {
	case "application/x-protobuf", "application/protobuf":
		td, err = Unmarshal(data)
	case "application/json":
		err = json.Unmarshal(data, &td)
	default:
		http.Error(w, "unsupported content type: "+ct, http.StatusUnsupportedMediaType)
		return
	}
	if err != nil {
		http.Error(w, "decode: "+err.Error(), http.StatusBadRequest)
		return
	}

	spanCount := countSpans(td)
	if err := r.sink.ConsumeTraces(req.Context(), td); err != nil {
		r.spansRejected.Add(int64(spanCount))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprintf(w, `{"partialSuccess":{"rejectedSpans":%d,"errorMessage":"%s"}}`, spanCount, err.Error())
		return
	}

	r.spansReceived.Add(int64(spanCount))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("{}"))
}

// SpansReceived returns the total spans received.
func (r *HTTPReceiver) SpansReceived() int64 {
	return r.spansReceived.Load()
}

// SpansRejected returns the total spans rejected.
func (r *HTTPReceiver) SpansRejected() int64 {
	return r.spansRejected.Load()
}

func countSpans(td TracesData) int {
	count := 0
	for _, rs := range td.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			count += len(ss.Spans)
		}
	}
	return count
}
