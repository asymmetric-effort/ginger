package influxdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
	"github.com/asymmetric-effort/ginger/internal/storage"
)

func TestTraceWriterWriteSpans(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient(ClientConfig{Endpoint: srv.URL, Org: "o", Bucket: "b"})
	tw := NewTraceWriter(client)

	resAttrs := otlp.NewAttributes()
	resAttrs.Set("service.name", otlp.StringValue("test-svc"))
	td := otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			Resource: otlp.Resource{Attributes: resAttrs},
			ScopeSpans: []otlp.ScopeSpans{{
				Scope: otlp.InstrumentationScope{Name: "lib"},
				Spans: []otlp.Span{{
					TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "op",
					Kind: otlp.SpanKindServer, Status: otlp.Status{Code: otlp.StatusCodeOk},
					StartTimeUnixNano: 1000000000, EndTimeUnixNano: 2000000000,
				}},
			}},
		}},
	}

	err := tw.WriteSpans(context.Background(), td)
	if err != nil {
		t.Fatal(err)
	}
}

func TestTraceWriterEmptyData(t *testing.T) {
	tw := NewTraceWriter(NewClient(ClientConfig{Endpoint: "http://localhost:1"}))
	err := tw.WriteSpans(context.Background(), otlp.TracesData{})
	if err != nil {
		t.Fatal(err) // empty should be no-op
	}
}

func TestTraceReaderGetServices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("_value\nfrontend\nbackend\n"))
	}))
	defer srv.Close()

	client := NewClient(ClientConfig{Endpoint: srv.URL, Org: "o", Bucket: "b"})
	tr := NewTraceReader(client, "b")

	services, err := tr.GetServices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(services) != 2 {
		t.Errorf("services = %d", len(services))
	}
}

func TestTraceReaderGetOperations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("operation,span_kind\nGET /api,SERVER\nPOST /api,SERVER\n"))
	}))
	defer srv.Close()

	client := NewClient(ClientConfig{Endpoint: srv.URL, Org: "o", Bucket: "b"})
	tr := NewTraceReader(client, "b")

	ops, err := tr.GetOperations(context.Background(), "svc")
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 2 {
		t.Errorf("ops = %d", len(ops))
	}
}

func TestDependencyStorageWriteLinks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient(ClientConfig{Endpoint: srv.URL, Org: "o", Bucket: "b"})
	ds := NewDependencyStorage(client, "b")

	err := ds.WriteLinks(context.Background(), time.Now(), []storage.DependencyLink{
		{Parent: "a", Child: "b", CallCount: 10},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDependencyStorageGetDependencies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("parent,child,_value\nfrontend,backend,42\n"))
	}))
	defer srv.Close()

	client := NewClient(ClientConfig{Endpoint: srv.URL, Org: "o", Bucket: "b"})
	ds := NewDependencyStorage(client, "b")

	deps, err := ds.GetDependencies(context.Background(), time.Now(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 1 {
		t.Errorf("deps = %d", len(deps))
	}
	if deps[0].CallCount != 42 {
		t.Errorf("call count = %d", deps[0].CallCount)
	}
}

func TestSamplingStorageInsertThroughput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient(ClientConfig{Endpoint: srv.URL, Org: "o", Bucket: "b"})
	ss := NewSamplingStorage(client, "b")

	err := ss.InsertThroughput(context.Background(), []storage.Throughput{
		{Service: "svc", Operation: "op", Count: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSamplingStorageInsertProbabilities(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient(ClientConfig{Endpoint: srv.URL, Org: "o", Bucket: "b"})
	ss := NewSamplingStorage(client, "b")

	probs := storage.ServiceOperationProbabilities{"svc": {"op": 0.5}}
	err := ss.InsertProbabilities(context.Background(), "host1", probs)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSamplingStorageGetThroughput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("service,operation\nsvc,op\n"))
	}))
	defer srv.Close()

	client := NewClient(ClientConfig{Endpoint: srv.URL, Org: "o", Bucket: "b"})
	ss := NewSamplingStorage(client, "b")

	throughput, err := ss.GetThroughput(context.Background(), time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(throughput) != 1 {
		t.Errorf("throughput = %d", len(throughput))
	}
}

func TestSamplingStorageGetLatestProbabilities(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`_value` + "\n" + `{"svc":{"op":0.5}}` + "\n"))
	}))
	defer srv.Close()

	client := NewClient(ClientConfig{Endpoint: srv.URL, Org: "o", Bucket: "b"})
	ss := NewSamplingStorage(client, "b")

	probs, err := ss.GetLatestProbabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if probs["svc"]["op"] != 0.5 {
		t.Errorf("probs = %v", probs)
	}
}

func TestSamplingStorageGetLatestEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(""))
	}))
	defer srv.Close()

	client := NewClient(ClientConfig{Endpoint: srv.URL, Org: "o", Bucket: "b"})
	ss := NewSamplingStorage(client, "b")

	probs, err := ss.GetLatestProbabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(probs) != 0 {
		t.Error("should be empty")
	}
}
