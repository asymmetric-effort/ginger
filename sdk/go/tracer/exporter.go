package tracer

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// OTLPHTTPExporter sends spans to an OTLP-compatible endpoint via HTTP POST.
type OTLPHTTPExporter struct {
	endpoint  string
	service   string
	batchSize int
	mu        sync.Mutex
	pending   []*Span
	client    *http.Client
}

// NewOTLPHTTPExporter creates a new OTLP HTTP exporter.
func NewOTLPHTTPExporter(endpoint, service string, batchSize int) *OTLPHTTPExporter {
	if batchSize <= 0 {
		batchSize = 512
	}
	return &OTLPHTTPExporter{
		endpoint:  endpoint,
		service:   service,
		batchSize: batchSize,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Export adds spans to the pending batch and flushes when the batch is full.
func (e *OTLPHTTPExporter) Export(spans ...*Span) error {
	e.mu.Lock()
	e.pending = append(e.pending, spans...)
	shouldFlush := len(e.pending) >= e.batchSize
	e.mu.Unlock()

	if shouldFlush {
		return e.Flush()
	}
	return nil
}

// Flush sends all pending spans to the OTLP endpoint.
func (e *OTLPHTTPExporter) Flush() error {
	e.mu.Lock()
	if len(e.pending) == 0 {
		e.mu.Unlock()
		return nil
	}
	spans := e.pending
	e.pending = nil
	e.mu.Unlock()

	payload := e.buildPayload(spans)
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("otlp: marshal: %w", err)
	}

	url := e.endpoint + "/v1/traces"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("otlp: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("otlp: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("otlp: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// Shutdown flushes any remaining spans.
func (e *OTLPHTTPExporter) Shutdown() error {
	return e.Flush()
}

// buildPayload constructs the OTLP JSON trace export request.
func (e *OTLPHTTPExporter) buildPayload(spans []*Span) map[string]interface{} {
	otlpSpans := make([]map[string]interface{}, 0, len(spans))
	for _, s := range spans {
		s.mu.Lock()
		otlpSpan := map[string]interface{}{
			"traceId":           hex.EncodeToString(s.traceID[:]),
			"spanId":            hex.EncodeToString(s.spanID[:]),
			"name":              s.name,
			"kind":              int(s.kind),
			"startTimeUnixNano": fmt.Sprintf("%d", s.startTime.UnixNano()),
			"endTimeUnixNano":   fmt.Sprintf("%d", s.endTime.UnixNano()),
			"attributes":        buildAttributes(s.attrs),
			"status":            buildStatus(s.status, s.statusMsg),
			"events":            buildEvents(s.events),
		}
		s.mu.Unlock()
		otlpSpans = append(otlpSpans, otlpSpan)
	}

	return map[string]interface{}{
		"resourceSpans": []map[string]interface{}{
			{
				"resource": map[string]interface{}{
					"attributes": []map[string]interface{}{
						{
							"key":   "service.name",
							"value": map[string]interface{}{"stringValue": e.service},
						},
					},
				},
				"scopeSpans": []map[string]interface{}{
					{
						"scope": map[string]interface{}{"name": "ginger-go"},
						"spans": otlpSpans,
					},
				},
			},
		},
	}
}

func buildAttributes(attrs map[string]interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(attrs))
	for k, v := range attrs {
		attr := map[string]interface{}{"key": k}
		switch val := v.(type) {
		case string:
			attr["value"] = map[string]interface{}{"stringValue": val}
		case int:
			attr["value"] = map[string]interface{}{"intValue": fmt.Sprintf("%d", val)}
		case int64:
			attr["value"] = map[string]interface{}{"intValue": fmt.Sprintf("%d", val)}
		case float64:
			attr["value"] = map[string]interface{}{"doubleValue": val}
		case bool:
			attr["value"] = map[string]interface{}{"boolValue": val}
		default:
			attr["value"] = map[string]interface{}{"stringValue": fmt.Sprintf("%v", val)}
		}
		result = append(result, attr)
	}
	return result
}

func buildStatus(code StatusCode, msg string) map[string]interface{} {
	s := map[string]interface{}{
		"code": int(code),
	}
	if msg != "" {
		s["message"] = msg
	}
	return s
}

func buildEvents(events []Event) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(events))
	for _, ev := range events {
		attrs := make([]map[string]interface{}, 0, len(ev.Attributes))
		for k, v := range ev.Attributes {
			attrs = append(attrs, map[string]interface{}{
				"key":   k,
				"value": map[string]interface{}{"stringValue": v},
			})
		}
		result = append(result, map[string]interface{}{
			"name":         ev.Name,
			"timeUnixNano": fmt.Sprintf("%d", ev.Timestamp.UnixNano()),
			"attributes":   attrs,
		})
	}
	return result
}
