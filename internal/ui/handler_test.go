package ui

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerServesIndex(t *testing.T) {
	h, err := NewHandler()
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	h.ServeHTTP(w, r)

	// Should serve index.html (or directory listing)
	if w.Code != http.StatusOK && w.Code != http.StatusMovedPermanently {
		t.Errorf("status = %d", w.Code)
	}
}

func TestHandlerAPIRoutesNotServed(t *testing.T) {
	h, _ := NewHandler()

	routes := []string{"/api/v3/services", "/v1/traces", "/health/live", "/debug/pprof/", "/sampling"}
	for _, route := range routes {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", route, nil)
		h.ServeHTTP(w, r)
		if w.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", route, w.Code)
		}
	}
}
