package query

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
	"github.com/asymmetric-effort/ginger/internal/storage"
	"github.com/asymmetric-effort/ginger/internal/storage/memory"
)

func setupTestService() (*Service, *memory.Backend) {
	backend := memory.NewBackend(100)
	svc := NewService(backend, backend, nil)
	return svc, backend
}

func seedData(t *testing.T, backend *memory.Backend) {
	t.Helper()
	ctx := context.Background()
	resAttrs := otlp.NewAttributes()
	resAttrs.Set("service.name", otlp.StringValue("test-svc"))

	td := otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			Resource: otlp.Resource{Attributes: resAttrs},
			ScopeSpans: []otlp.ScopeSpans{{
				Spans: []otlp.Span{{
					TraceID: [16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
						0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
					SpanID:            [8]byte{1},
					Name:              "GET /api",
					Kind:              otlp.SpanKindServer,
					StartTimeUnixNano: uint64(time.Now().UnixNano()),
					EndTimeUnixNano:   uint64(time.Now().Add(time.Second).UnixNano()),
				}},
			}},
		}},
	}
	backend.WriteSpans(ctx, td)
}

func TestV3GetTrace(t *testing.T) {
	svc, backend := setupTestService()
	seedData(t, backend)
	h := NewHTTPHandler(svc)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v3/traces/0102030405060708090a0b0c0d0e0f10", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestV3GetTraceNotFound(t *testing.T) {
	svc, _ := setupTestService()
	h := NewHTTPHandler(svc)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v3/traces/ffffffffffffffffffffffffffffffff", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d", w.Code)
	}
}

func TestV3GetTraceInvalidID(t *testing.T) {
	svc, _ := setupTestService()
	h := NewHTTPHandler(svc)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v3/traces/invalid", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}
}

func TestV3GetTraceEmpty(t *testing.T) {
	svc, _ := setupTestService()
	h := NewHTTPHandler(svc)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v3/traces/", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}
}

func TestV3FindTraces(t *testing.T) {
	svc, backend := setupTestService()
	seedData(t, backend)
	h := NewHTTPHandler(svc)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v3/traces?service=test-svc&limit=10", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
}

func TestV3GetServices(t *testing.T) {
	svc, backend := setupTestService()
	seedData(t, backend)
	h := NewHTTPHandler(svc)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v3/services", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
	var services []string
	json.Unmarshal(w.Body.Bytes(), &services)
	if len(services) != 1 || services[0] != "test-svc" {
		t.Errorf("services = %v", services)
	}
}

func TestV3GetOperations(t *testing.T) {
	svc, backend := setupTestService()
	seedData(t, backend)
	h := NewHTTPHandler(svc)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v3/operations?service=test-svc", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
}

func TestV3GetOperationsMissing(t *testing.T) {
	svc, _ := setupTestService()
	h := NewHTTPHandler(svc)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v3/operations", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}
}

func TestV3GetDependencies(t *testing.T) {
	svc, backend := setupTestService()
	ctx := context.Background()
	backend.WriteLinks(ctx, time.Now(), []storage.DependencyLink{
		{Parent: "a", Child: "b", CallCount: 5},
	})
	h := NewHTTPHandler(svc)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v3/dependencies", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
}

func TestV3GetDependenciesWithParams(t *testing.T) {
	svc, _ := setupTestService()
	h := NewHTTPHandler(svc)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	now := time.Now().UnixMilli()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v3/dependencies?endTs="+itoa(now)+"&lookback=3600000", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
}

func TestV3GetDependenciesBadEndTs(t *testing.T) {
	svc, _ := setupTestService()
	h := NewHTTPHandler(svc)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v3/dependencies?endTs=bad", nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}
}

func TestV3GetDependenciesBadLookback(t *testing.T) {
	svc, _ := setupTestService()
	h := NewHTTPHandler(svc)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v3/dependencies?lookback=bad", nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}
}

// === v2 Legacy API Tests ===

func TestV2GetTrace(t *testing.T) {
	svc, backend := setupTestService()
	seedData(t, backend)
	h := NewHTTPHandler(svc)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/traces/0102030405060708090a0b0c0d0e0f10", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
	var resp v2Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Errors) != 0 {
		t.Errorf("errors = %v", resp.Errors)
	}
}

func TestV2GetTraceNotFound(t *testing.T) {
	svc, _ := setupTestService()
	h := NewHTTPHandler(svc)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/traces/ffffffffffffffffffffffffffffffff", nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d", w.Code)
	}
}

func TestV2GetTraceEmpty(t *testing.T) {
	svc, _ := setupTestService()
	h := NewHTTPHandler(svc)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/traces/", nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}
}

func TestV2GetTraceInvalid(t *testing.T) {
	svc, _ := setupTestService()
	h := NewHTTPHandler(svc)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/traces/xyz", nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}
}

func TestV2FindTraces(t *testing.T) {
	svc, backend := setupTestService()
	seedData(t, backend)
	h := NewHTTPHandler(svc)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/traces?service=test-svc", nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
}

func TestV2GetServices(t *testing.T) {
	svc, backend := setupTestService()
	seedData(t, backend)
	h := NewHTTPHandler(svc)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/services", nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
}

func TestV2GetOperations(t *testing.T) {
	svc, backend := setupTestService()
	seedData(t, backend)
	h := NewHTTPHandler(svc)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/operations?service=test-svc", nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
}

func TestV2GetDependencies(t *testing.T) {
	svc, _ := setupTestService()
	h := NewHTTPHandler(svc)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/dependencies", nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
}

func TestParseTagsJSON(t *testing.T) {
	tags := parseTags(`{"key":"val"}`)
	if tags["key"] != "val" {
		t.Errorf("tags = %v", tags)
	}
}

func TestParseTagsPipe(t *testing.T) {
	tags := parseTags("k1:v1|k2:v2")
	if tags["k1"] != "v1" || tags["k2"] != "v2" {
		t.Errorf("tags = %v", tags)
	}
}

func TestFindTracesWithAllParams(t *testing.T) {
	svc, backend := setupTestService()
	seedData(t, backend)
	h := NewHTTPHandler(svc)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v3/traces?service=test-svc&operation=GET+/api&limit=5&start=1&end=999999999999999&minDuration=1ms&maxDuration=10s&tags=k:v", nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
}

func TestServiceGetTrace(t *testing.T) {
	svc, backend := setupTestService()
	seedData(t, backend)

	tid := [16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}
	td, err := svc.GetTrace(context.Background(), tid)
	if err != nil {
		t.Fatal(err)
	}
	if len(td.ResourceSpans) == 0 {
		t.Error("should return data")
	}
}

func TestServiceGetTraceNotFound(t *testing.T) {
	svc, _ := setupTestService()
	_, err := svc.GetTrace(context.Background(), [16]byte{99})
	if err != storage.ErrTraceNotFound {
		t.Errorf("expected ErrTraceNotFound, got %v", err)
	}
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
