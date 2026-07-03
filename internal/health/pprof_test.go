package health

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterPprof(t *testing.T) {
	mux := http.NewServeMux()
	RegisterPprof(mux)

	// Verify index is registered
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/debug/pprof/", nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("pprof index: status = %d", w.Code)
	}

	// Verify cmdline
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/debug/pprof/cmdline", nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("pprof cmdline: status = %d", w.Code)
	}

	// Verify symbol
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/debug/pprof/symbol", nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("pprof symbol: status = %d", w.Code)
	}
}
