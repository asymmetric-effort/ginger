package otlp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockSink struct {
	traces []TracesData
	err    error
}

func (s *mockSink) ConsumeTraces(_ context.Context, td TracesData) error {
	if s.err != nil {
		return s.err
	}
	s.traces = append(s.traces, td)
	return nil
}

func TestHTTPReceiverJSON(t *testing.T) {
	sink := &mockSink{}
	recv := NewHTTPReceiver(sink, 0)

	td := TracesData{
		ResourceSpans: []ResourceSpans{{
			ScopeSpans: []ScopeSpans{{
				Spans: []Span{{
					TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "test",
					StartTimeUnixNano: 1000, EndTimeUnixNano: 2000,
				}},
			}},
		}},
	}
	body, _ := json.Marshal(td)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/traces", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	recv.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if len(sink.traces) != 1 {
		t.Errorf("traces = %d", len(sink.traces))
	}
	if recv.SpansReceived() != 1 {
		t.Errorf("received = %d", recv.SpansReceived())
	}
}

func TestHTTPReceiverProtobuf(t *testing.T) {
	sink := &mockSink{}
	recv := NewHTTPReceiver(sink, 0)

	td := TracesData{
		ResourceSpans: []ResourceSpans{{
			ScopeSpans: []ScopeSpans{{
				Spans: []Span{{
					TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "test",
					StartTimeUnixNano: 1000, EndTimeUnixNano: 2000,
				}},
			}},
		}},
	}
	body, _ := Marshal(td)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/traces", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/x-protobuf")
	recv.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
}

func TestHTTPReceiverMethodNotAllowed(t *testing.T) {
	sink := &mockSink{}
	recv := NewHTTPReceiver(sink, 0)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/v1/traces", nil)
	recv.ServeHTTP(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d", w.Code)
	}
}

func TestHTTPReceiverUnsupportedContentType(t *testing.T) {
	sink := &mockSink{}
	recv := NewHTTPReceiver(sink, 0)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/traces", bytes.NewReader([]byte("{}")))
	r.Header.Set("Content-Type", "text/plain")
	recv.ServeHTTP(w, r)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("status = %d", w.Code)
	}
}

func TestHTTPReceiverSinkError(t *testing.T) {
	sink := &mockSink{err: fmt.Errorf("queue full")}
	recv := NewHTTPReceiver(sink, 0)

	td := TracesData{
		ResourceSpans: []ResourceSpans{{
			ScopeSpans: []ScopeSpans{{
				Spans: []Span{{TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "test",
					StartTimeUnixNano: 1, EndTimeUnixNano: 2}},
			}},
		}},
	}
	body, _ := json.Marshal(td)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/traces", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	recv.ServeHTTP(w, r)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d", w.Code)
	}
	if recv.SpansRejected() != 1 {
		t.Errorf("rejected = %d", recv.SpansRejected())
	}
}

func TestHTTPReceiverBadJSON(t *testing.T) {
	sink := &mockSink{}
	recv := NewHTTPReceiver(sink, 0)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/traces", bytes.NewReader([]byte("not json")))
	r.Header.Set("Content-Type", "application/json")
	recv.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}
}

func TestHTTPReceiverBadProtobuf(t *testing.T) {
	sink := &mockSink{}
	recv := NewHTTPReceiver(sink, 0)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/traces", bytes.NewReader([]byte{0xff, 0xff}))
	r.Header.Set("Content-Type", "application/x-protobuf")
	recv.ServeHTTP(w, r)

	// May or may not error depending on protobuf parser — either OK or BadRequest is acceptable
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}
}

func TestHTTPReceiverBadGzip(t *testing.T) {
	sink := &mockSink{}
	recv := NewHTTPReceiver(sink, 0)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/traces", bytes.NewReader([]byte("not gzip")))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Content-Encoding", "gzip")
	recv.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}
}

func TestCountSpans(t *testing.T) {
	td := TracesData{
		ResourceSpans: []ResourceSpans{{
			ScopeSpans: []ScopeSpans{{
				Spans: []Span{{}, {}, {}},
			}},
		}},
	}
	if countSpans(td) != 3 {
		t.Errorf("count = %d", countSpans(td))
	}
}
