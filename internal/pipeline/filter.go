package pipeline

import (
	"context"
	"errors"
	"regexp"
	"sync/atomic"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

var (
	// ErrMemoryLimitExceeded is returned when the memory limiter drops a batch.
	ErrMemoryLimitExceeded = errors.New("memory limit exceeded")
)

// FilterMode determines whether filter includes or excludes matching spans.
type FilterMode int

const (
	FilterModeInclude FilterMode = iota
	FilterModeExclude
)

// FilterConfig configures the filter processor.
type FilterConfig struct {
	Mode          FilterMode
	ServiceGlob   string
	SpanNameGlob  string
	SpanKind      otlp.SpanKind
	MatchSpanKind bool
	Attributes    map[string]string // key=value exact match
}

// FilterProcessor drops or keeps spans based on filter predicates.
type FilterProcessor struct {
	config       FilterConfig
	serviceRE    *regexp.Regexp
	spanNameRE   *regexp.Regexp
	droppedCount atomic.Int64
}

// NewFilterProcessor creates a new filter processor.
func NewFilterProcessor(config FilterConfig) *FilterProcessor {
	fp := &FilterProcessor{config: config}
	if config.ServiceGlob != "" {
		fp.serviceRE = globToRegexp(config.ServiceGlob)
	}
	if config.SpanNameGlob != "" {
		fp.spanNameRE = globToRegexp(config.SpanNameGlob)
	}
	return fp
}

// ProcessTraces filters spans.
func (fp *FilterProcessor) ProcessTraces(_ context.Context, td otlp.TracesData) (otlp.TracesData, error) {
	var result otlp.TracesData
	for _, rs := range td.ResourceSpans {
		svcName := ""
		if v, ok := rs.Resource.Attributes.Get("service.name"); ok {
			svcName = v.Str
		}

		var filteredScopeSpans []otlp.ScopeSpans
		for _, ss := range rs.ScopeSpans {
			var kept []otlp.Span
			for _, span := range ss.Spans {
				matches := fp.matches(svcName, span)
				if fp.config.Mode == FilterModeInclude {
					if matches {
						kept = append(kept, span)
					} else {
						fp.droppedCount.Add(1)
					}
				} else {
					if !matches {
						kept = append(kept, span)
					} else {
						fp.droppedCount.Add(1)
					}
				}
			}
			if len(kept) > 0 {
				filteredScopeSpans = append(filteredScopeSpans, otlp.ScopeSpans{Scope: ss.Scope, Spans: kept})
			}
		}
		if len(filteredScopeSpans) > 0 {
			result.ResourceSpans = append(result.ResourceSpans, otlp.ResourceSpans{
				Resource:   rs.Resource,
				ScopeSpans: filteredScopeSpans,
			})
		}
	}
	return result, nil
}

// DroppedCount returns the number of dropped spans.
func (fp *FilterProcessor) DroppedCount() int64 {
	return fp.droppedCount.Load()
}

func (fp *FilterProcessor) matches(service string, span otlp.Span) bool {
	if fp.serviceRE != nil && !fp.serviceRE.MatchString(service) {
		return false
	}
	if fp.spanNameRE != nil && !fp.spanNameRE.MatchString(span.Name) {
		return false
	}
	if fp.config.MatchSpanKind && span.Kind != fp.config.SpanKind {
		return false
	}
	for k, v := range fp.config.Attributes {
		av, ok := span.Attributes.Get(k)
		if !ok || av.Str != v {
			return false
		}
	}
	return true
}

func globToRegexp(glob string) *regexp.Regexp {
	// Simple glob: * → .*, ? → .
	pattern := "^"
	for _, c := range glob {
		switch c {
		case '*':
			pattern += ".*"
		case '?':
			pattern += "."
		case '.', '(', ')', '+', '{', '}', '[', ']', '^', '$', '|', '\\':
			pattern += `\` + string(c)
		default:
			pattern += string(c)
		}
	}
	pattern += "$"
	return regexp.MustCompile(pattern)
}

// AttributesProcessor modifies span attributes.
type AttributesProcessor struct {
	actions []AttributeAction
}

// AttributeActionType identifies the action to perform.
type AttributeActionType int

const (
	ActionInsert AttributeActionType = iota
	ActionUpdate
	ActionUpsert
	ActionDelete
)

// AttributeAction describes a single attribute modification.
type AttributeAction struct {
	Type  AttributeActionType
	Key   string
	Value otlp.AnyValue
}

// NewAttributesProcessor creates a new attributes processor.
func NewAttributesProcessor(actions []AttributeAction) *AttributesProcessor {
	return &AttributesProcessor{actions: actions}
}

// ProcessTraces modifies span attributes according to configured actions.
func (ap *AttributesProcessor) ProcessTraces(_ context.Context, td otlp.TracesData) (otlp.TracesData, error) {
	for ri := range td.ResourceSpans {
		for si := range td.ResourceSpans[ri].ScopeSpans {
			for spi := range td.ResourceSpans[ri].ScopeSpans[si].Spans {
				span := &td.ResourceSpans[ri].ScopeSpans[si].Spans[spi]
				for _, action := range ap.actions {
					ap.applyAction(span, action)
				}
			}
		}
	}
	return td, nil
}

func (ap *AttributesProcessor) applyAction(span *otlp.Span, action AttributeAction) {
	switch action.Type {
	case ActionInsert:
		if _, exists := span.Attributes.Get(action.Key); !exists {
			span.Attributes.Set(action.Key, action.Value)
		}
	case ActionUpdate:
		if _, exists := span.Attributes.Get(action.Key); exists {
			span.Attributes.Set(action.Key, action.Value)
		}
	case ActionUpsert:
		span.Attributes.Set(action.Key, action.Value)
	case ActionDelete:
		// Create new attributes without the key
		items := span.Attributes.Items()
		newAttrs := otlp.NewAttributes()
		for _, kv := range items {
			if kv.Key != action.Key {
				newAttrs.Set(kv.Key, kv.Value)
			}
		}
		span.Attributes = newAttrs
	}
}
