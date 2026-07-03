package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

func makeFilterTD(service, spanName string, kind otlp.SpanKind, attrs map[string]string) otlp.TracesData {
	resAttrs := otlp.NewAttributes()
	if service != "" {
		resAttrs.Set("service.name", otlp.StringValue(service))
	}
	spanAttrs := otlp.NewAttributes()
	for k, v := range attrs {
		spanAttrs.Set(k, otlp.StringValue(v))
	}
	return otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			Resource: otlp.Resource{Attributes: resAttrs},
			ScopeSpans: []otlp.ScopeSpans{{
				Spans: []otlp.Span{{
					TraceID: [16]byte{1}, SpanID: [8]byte{1},
					Name: spanName, Kind: kind,
					StartTimeUnixNano: 1000, EndTimeUnixNano: 2000,
					Attributes: spanAttrs,
				}},
			}},
		}},
	}
}

func spanCount(td otlp.TracesData) int {
	count := 0
	for _, rs := range td.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			count += len(ss.Spans)
		}
	}
	return count
}

func TestFilterExcludeByService(t *testing.T) {
	fp := NewFilterProcessor(FilterConfig{
		Mode:        FilterModeExclude,
		ServiceGlob: "internal-*",
	})
	td := makeFilterTD("internal-monitor", "op", otlp.SpanKindServer, nil)
	result, _ := fp.ProcessTraces(context.Background(), td)
	if spanCount(result) != 0 {
		t.Error("should exclude matching service")
	}
	if fp.DroppedCount() != 1 {
		t.Errorf("dropped = %d", fp.DroppedCount())
	}
}

func TestFilterIncludeByService(t *testing.T) {
	fp := NewFilterProcessor(FilterConfig{
		Mode:        FilterModeInclude,
		ServiceGlob: "frontend*",
	})
	ctx := context.Background()

	result, _ := fp.ProcessTraces(ctx, makeFilterTD("frontend-web", "op", 0, nil))
	if spanCount(result) != 1 {
		t.Error("should include matching service")
	}

	result, _ = fp.ProcessTraces(ctx, makeFilterTD("backend", "op", 0, nil))
	if spanCount(result) != 0 {
		t.Error("should exclude non-matching service")
	}
}

func TestFilterBySpanName(t *testing.T) {
	fp := NewFilterProcessor(FilterConfig{
		Mode:         FilterModeExclude,
		SpanNameGlob: "health*",
	})
	result, _ := fp.ProcessTraces(context.Background(), makeFilterTD("svc", "healthcheck", 0, nil))
	if spanCount(result) != 0 {
		t.Error("should exclude matching span name")
	}
}

func TestFilterBySpanKind(t *testing.T) {
	fp := NewFilterProcessor(FilterConfig{
		Mode:          FilterModeInclude,
		MatchSpanKind: true,
		SpanKind:      otlp.SpanKindServer,
	})
	ctx := context.Background()

	result, _ := fp.ProcessTraces(ctx, makeFilterTD("svc", "op", otlp.SpanKindServer, nil))
	if spanCount(result) != 1 {
		t.Error("should include server spans")
	}

	result, _ = fp.ProcessTraces(ctx, makeFilterTD("svc", "op", otlp.SpanKindClient, nil))
	if spanCount(result) != 0 {
		t.Error("should exclude client spans")
	}
}

func TestFilterByAttributes(t *testing.T) {
	fp := NewFilterProcessor(FilterConfig{
		Mode:       FilterModeInclude,
		Attributes: map[string]string{"env": "prod"},
	})
	ctx := context.Background()

	result, _ := fp.ProcessTraces(ctx, makeFilterTD("svc", "op", 0, map[string]string{"env": "prod"}))
	if spanCount(result) != 1 {
		t.Error("should include matching attribute")
	}

	result, _ = fp.ProcessTraces(ctx, makeFilterTD("svc", "op", 0, map[string]string{"env": "dev"}))
	if spanCount(result) != 0 {
		t.Error("should exclude non-matching attribute")
	}
}

func TestAttributesProcessorInsert(t *testing.T) {
	ap := NewAttributesProcessor([]AttributeAction{
		{Type: ActionInsert, Key: "new_key", Value: otlp.StringValue("new_val")},
	})
	td := makeFilterTD("svc", "op", 0, nil)
	result, _ := ap.ProcessTraces(context.Background(), td)
	span := result.ResourceSpans[0].ScopeSpans[0].Spans[0]
	v, ok := span.Attributes.Get("new_key")
	if !ok || v.Str != "new_val" {
		t.Error("insert should add new key")
	}
}

func TestAttributesProcessorInsertExisting(t *testing.T) {
	ap := NewAttributesProcessor([]AttributeAction{
		{Type: ActionInsert, Key: "existing", Value: otlp.StringValue("new")},
	})
	td := makeFilterTD("svc", "op", 0, map[string]string{"existing": "old"})
	result, _ := ap.ProcessTraces(context.Background(), td)
	span := result.ResourceSpans[0].ScopeSpans[0].Spans[0]
	v, _ := span.Attributes.Get("existing")
	if v.Str != "old" {
		t.Error("insert should not overwrite existing")
	}
}

func TestAttributesProcessorUpdate(t *testing.T) {
	ap := NewAttributesProcessor([]AttributeAction{
		{Type: ActionUpdate, Key: "existing", Value: otlp.StringValue("updated")},
	})
	td := makeFilterTD("svc", "op", 0, map[string]string{"existing": "old"})
	result, _ := ap.ProcessTraces(context.Background(), td)
	span := result.ResourceSpans[0].ScopeSpans[0].Spans[0]
	v, _ := span.Attributes.Get("existing")
	if v.Str != "updated" {
		t.Error("update should overwrite existing")
	}
}

func TestAttributesProcessorUpdateMissing(t *testing.T) {
	ap := NewAttributesProcessor([]AttributeAction{
		{Type: ActionUpdate, Key: "missing", Value: otlp.StringValue("val")},
	})
	td := makeFilterTD("svc", "op", 0, nil)
	result, _ := ap.ProcessTraces(context.Background(), td)
	span := result.ResourceSpans[0].ScopeSpans[0].Spans[0]
	_, ok := span.Attributes.Get("missing")
	if ok {
		t.Error("update should not add missing key")
	}
}

func TestAttributesProcessorUpsert(t *testing.T) {
	ap := NewAttributesProcessor([]AttributeAction{
		{Type: ActionUpsert, Key: "key", Value: otlp.StringValue("val")},
	})
	td := makeFilterTD("svc", "op", 0, nil)
	result, _ := ap.ProcessTraces(context.Background(), td)
	span := result.ResourceSpans[0].ScopeSpans[0].Spans[0]
	v, ok := span.Attributes.Get("key")
	if !ok || v.Str != "val" {
		t.Error("upsert should add or update")
	}
}

func TestAttributesProcessorDelete(t *testing.T) {
	ap := NewAttributesProcessor([]AttributeAction{
		{Type: ActionDelete, Key: "to_delete"},
	})
	td := makeFilterTD("svc", "op", 0, map[string]string{"to_delete": "val", "keep": "val2"})
	result, _ := ap.ProcessTraces(context.Background(), td)
	span := result.ResourceSpans[0].ScopeSpans[0].Spans[0]
	_, ok := span.Attributes.Get("to_delete")
	if ok {
		t.Error("delete should remove key")
	}
	_, ok = span.Attributes.Get("keep")
	if !ok {
		t.Error("should keep other keys")
	}
}

func TestMemoryLimiterHardLimit(t *testing.T) {
	ml := NewMemoryLimiter(MemoryLimiterConfig{HardLimitMiB: 1}) // 1 MiB — will be exceeded
	ml.hardExceed.Store(true) // simulate exceeded

	td := makeFilterTD("svc", "op", 0, nil)
	_, err := ml.ProcessTraces(context.Background(), td)
	if err != ErrMemoryLimitExceeded {
		t.Errorf("expected ErrMemoryLimitExceeded, got %v", err)
	}
	if ml.DroppedCount() != 1 {
		t.Errorf("dropped = %d", ml.DroppedCount())
	}
}

func TestMemoryLimiterSoftLimit(t *testing.T) {
	ml := NewMemoryLimiter(MemoryLimiterConfig{SoftLimitMiB: 1, DropRatio: 0.5})
	ml.softExceed.Store(true)

	dropped := 0
	for i := 0; i < 10; i++ {
		td := makeFilterTD("svc", "op", 0, nil)
		_, err := ml.ProcessTraces(context.Background(), td)
		if err == ErrMemoryLimitExceeded {
			dropped++
		}
	}
	// With 50% drop ratio, should drop some but not all
	if dropped == 0 || dropped == 10 {
		t.Errorf("soft limit should drop some: dropped %d/10", dropped)
	}
}

func TestMemoryLimiterNormal(t *testing.T) {
	ml := NewMemoryLimiter(MemoryLimiterConfig{HardLimitMiB: 99999})
	td := makeFilterTD("svc", "op", 0, nil)
	result, err := ml.ProcessTraces(context.Background(), td)
	if err != nil {
		t.Errorf("normal operation should pass: %v", err)
	}
	if spanCount(result) != 1 {
		t.Error("should pass through")
	}
}

func TestMemoryLimiterStartShutdown(t *testing.T) {
	ml := NewMemoryLimiter(MemoryLimiterConfig{CheckInterval: 50 * time.Millisecond})
	ctx := context.Background()
	ml.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	ml.Shutdown()
	// Should not panic or deadlock
}

func TestMemoryLimiterDefaults(t *testing.T) {
	ml := NewMemoryLimiter(MemoryLimiterConfig{})
	if ml.config.CheckInterval != time.Second {
		t.Error("default CheckInterval")
	}
	if ml.config.DropRatio != 0.5 {
		t.Error("default DropRatio")
	}
}

func TestGlobToRegexp(t *testing.T) {
	tests := []struct {
		glob  string
		match string
		want  bool
	}{
		{"svc*", "svc-web", true},
		{"svc*", "other", false},
		{"*.api", "frontend.api", true},
		{"exact", "exact", true},
		{"exact", "not-exact", false},
		{"test?", "test1", true},
		{"test?", "test12", false},
	}
	for _, tt := range tests {
		re := globToRegexp(tt.glob)
		if re.MatchString(tt.match) != tt.want {
			t.Errorf("glob %q match %q = %v, want %v", tt.glob, tt.match, !tt.want, tt.want)
		}
	}
}
