package query

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/asymmetric-effort/ginger/internal/storage"
)

// HTTPHandler serves the Query API v3 REST endpoints.
type HTTPHandler struct {
	service *Service
}

// NewHTTPHandler creates a new HTTP handler for the query API.
func NewHTTPHandler(service *Service) *HTTPHandler {
	return &HTTPHandler{service: service}
}

// RegisterRoutes registers all query API routes on the given mux.
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v3/traces/", h.handleGetTrace)
	mux.HandleFunc("/api/v3/traces", h.handleFindTraces)
	mux.HandleFunc("/api/v3/services", h.handleGetServices)
	mux.HandleFunc("/api/v3/operations", h.handleGetOperations)
	mux.HandleFunc("/api/v3/dependencies", h.handleGetDependencies)

	// Legacy v2 API
	mux.HandleFunc("/api/traces/", h.handleV2GetTrace)
	mux.HandleFunc("/api/traces", h.handleV2FindTraces)
	mux.HandleFunc("/api/services", h.handleV2GetServices)
	mux.HandleFunc("/api/operations", h.handleV2GetOperations)
	mux.HandleFunc("/api/dependencies", h.handleV2GetDependencies)
}

// === v3 API ===

func (h *HTTPHandler) handleGetTrace(w http.ResponseWriter, r *http.Request) {
	traceIDStr := strings.TrimPrefix(r.URL.Path, "/api/v3/traces/")
	if traceIDStr == "" {
		writeJSONError(w, http.StatusBadRequest, "trace ID required")
		return
	}

	traceID, err := parseTraceID(traceIDStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid trace ID: "+err.Error())
		return
	}

	td, err := h.service.GetTrace(r.Context(), traceID)
	if err != nil {
		if err == storage.ErrTraceNotFound {
			writeJSONError(w, http.StatusNotFound, "trace not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, td)
}

func (h *HTTPHandler) handleFindTraces(w http.ResponseWriter, r *http.Request) {
	query := parseQueryParams(r)

	results, err := h.service.FindTraces(r.Context(), query)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, results)
}

func (h *HTTPHandler) handleGetServices(w http.ResponseWriter, r *http.Request) {
	services, err := h.service.GetServices(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, services)
}

func (h *HTTPHandler) handleGetOperations(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	if service == "" {
		writeJSONError(w, http.StatusBadRequest, "service parameter required")
		return
	}

	ops, err := h.service.GetOperations(r.Context(), service)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ops)
}

func (h *HTTPHandler) handleGetDependencies(w http.ResponseWriter, r *http.Request) {
	endTsStr := r.URL.Query().Get("endTs")
	lookbackStr := r.URL.Query().Get("lookback")

	endTs := time.Now()
	if endTsStr != "" {
		ms, err := strconv.ParseInt(endTsStr, 10, 64)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid endTs")
			return
		}
		endTs = time.UnixMilli(ms)
	}

	lookback := time.Hour
	if lookbackStr != "" {
		ms, err := strconv.ParseInt(lookbackStr, 10, 64)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid lookback")
			return
		}
		lookback = time.Duration(ms) * time.Millisecond
	}

	deps, err := h.service.GetDependencies(r.Context(), endTs, lookback)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, deps)
}

// === v2 Legacy API ===

type v2Response struct {
	Data   interface{} `json:"data"`
	Total  int         `json:"total,omitempty"`
	Limit  int         `json:"limit,omitempty"`
	Offset int         `json:"offset,omitempty"`
	Errors []string    `json:"errors"`
}

func (h *HTTPHandler) handleV2GetTrace(w http.ResponseWriter, r *http.Request) {
	traceIDStr := strings.TrimPrefix(r.URL.Path, "/api/traces/")
	if traceIDStr == "" {
		writeJSON(w, http.StatusBadRequest, v2Response{Errors: []string{"trace ID required"}})
		return
	}

	traceID, err := parseTraceID(traceIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, v2Response{Errors: []string{err.Error()}})
		return
	}

	td, err := h.service.GetTrace(r.Context(), traceID)
	if err != nil {
		if err == storage.ErrTraceNotFound {
			writeJSON(w, http.StatusNotFound, v2Response{Data: nil, Errors: []string{"trace not found"}})
			return
		}
		writeJSON(w, http.StatusInternalServerError, v2Response{Errors: []string{err.Error()}})
		return
	}

	writeJSON(w, http.StatusOK, v2Response{Data: []interface{}{td}, Errors: []string{}})
}

func (h *HTTPHandler) handleV2FindTraces(w http.ResponseWriter, r *http.Request) {
	query := parseQueryParams(r)

	results, err := h.service.FindTraces(r.Context(), query)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, v2Response{Errors: []string{err.Error()}})
		return
	}

	data := make([]interface{}, len(results))
	for i, r := range results {
		data[i] = r
	}

	writeJSON(w, http.StatusOK, v2Response{
		Data:   data,
		Total:  len(results),
		Limit:  query.NumTraces,
		Errors: []string{},
	})
}

func (h *HTTPHandler) handleV2GetServices(w http.ResponseWriter, r *http.Request) {
	services, err := h.service.GetServices(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, v2Response{Errors: []string{err.Error()}})
		return
	}
	writeJSON(w, http.StatusOK, v2Response{Data: services, Errors: []string{}})
}

func (h *HTTPHandler) handleV2GetOperations(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	ops, err := h.service.GetOperations(r.Context(), service)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, v2Response{Errors: []string{err.Error()}})
		return
	}
	writeJSON(w, http.StatusOK, v2Response{Data: ops, Errors: []string{}})
}

func (h *HTTPHandler) handleV2GetDependencies(w http.ResponseWriter, r *http.Request) {
	endTsStr := r.URL.Query().Get("endTs")
	lookbackStr := r.URL.Query().Get("lookback")

	endTs := time.Now()
	if endTsStr != "" {
		ms, _ := strconv.ParseInt(endTsStr, 10, 64)
		endTs = time.UnixMilli(ms)
	}
	lookback := time.Hour
	if lookbackStr != "" {
		ms, _ := strconv.ParseInt(lookbackStr, 10, 64)
		lookback = time.Duration(ms) * time.Millisecond
	}

	deps, err := h.service.GetDependencies(r.Context(), endTs, lookback)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, v2Response{Errors: []string{err.Error()}})
		return
	}
	writeJSON(w, http.StatusOK, v2Response{Data: deps, Errors: []string{}})
}

// === Helpers ===

func parseTraceID(s string) (storage.TraceID, error) {
	var tid storage.TraceID
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 16 {
		return tid, storage.ErrTraceNotFound
	}
	copy(tid[:], b)
	return tid, nil
}

func parseQueryParams(r *http.Request) storage.TraceQueryParameters {
	q := r.URL.Query()
	query := storage.TraceQueryParameters{
		ServiceName:   q.Get("service"),
		OperationName: q.Get("operation"),
	}

	if limit := q.Get("limit"); limit != "" {
		n, _ := strconv.Atoi(limit)
		query.NumTraces = n
	}

	if startMin := q.Get("start"); startMin != "" {
		us, _ := strconv.ParseInt(startMin, 10, 64)
		query.StartTimeMin = time.UnixMicro(us)
	}

	if startMax := q.Get("end"); startMax != "" {
		us, _ := strconv.ParseInt(startMax, 10, 64)
		query.StartTimeMax = time.UnixMicro(us)
	}

	if minDur := q.Get("minDuration"); minDur != "" {
		d, _ := time.ParseDuration(minDur)
		query.DurationMin = d
	}

	if maxDur := q.Get("maxDuration"); maxDur != "" {
		d, _ := time.ParseDuration(maxDur)
		query.DurationMax = d
	}

	if tags := q.Get("tags"); tags != "" {
		query.Tags = parseTags(tags)
	}

	return query
}

func parseTags(s string) map[string]string {
	tags := make(map[string]string)
	// Format: key1:value1|key2:value2 or JSON
	if strings.HasPrefix(s, "{") {
		json.Unmarshal([]byte(s), &tags)
		return tags
	}
	pairs := strings.Split(s, "|")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, ":", 2)
		if len(kv) == 2 {
			tags[kv[0]] = kv[1]
		}
	}
	return tags
}

func writeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func writeJSONError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
