package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
)

// Status represents a health status.
type Status string

const (
	StatusLive     Status = "live"
	StatusReady    Status = "ready"
	StatusNotReady Status = "not_ready"
)

// Checker is implemented by subsystems that can report health.
type Checker interface {
	Check(ctx context.Context) error
}

// CheckerFunc adapts a function to the Checker interface.
type CheckerFunc func(ctx context.Context) error

// Check implements Checker.
func (f CheckerFunc) Check(ctx context.Context) error {
	return f(ctx)
}

// Handler provides HTTP health check endpoints.
type Handler struct {
	mu       sync.RWMutex
	checkers map[string]Checker
	ready    bool
}

// NewHandler creates a new health Handler.
func NewHandler() *Handler {
	return &Handler{
		checkers: make(map[string]Checker),
	}
}

// Register adds a named health checker.
func (h *Handler) Register(name string, checker Checker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checkers[name] = checker
}

// SetReady marks the system as ready.
func (h *Handler) SetReady(ready bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ready = ready
}

// LiveHandler returns the liveness probe handler.
func (h *Handler) LiveHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": string(StatusLive)})
	}
}

// ReadyHandler returns the readiness probe handler.
func (h *Handler) ReadyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.mu.RLock()
		isReady := h.ready
		checkers := make(map[string]Checker, len(h.checkers))
		for k, v := range h.checkers {
			checkers[k] = v
		}
		h.mu.RUnlock()

		if !isReady {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": string(StatusNotReady)})
			return
		}

		// Run all checkers
		failures := make(map[string]string)
		for name, checker := range checkers {
			if err := checker.Check(r.Context()); err != nil {
				failures[name] = err.Error()
			}
		}

		if len(failures) > 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":   string(StatusNotReady),
				"failures": failures,
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": string(StatusReady)})
	}
}
