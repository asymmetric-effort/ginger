package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLiveHandler(t *testing.T) {
	h := NewHandler()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/health/live", nil)
	h.LiveHandler()(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "live" {
		t.Errorf("status = %q", body["status"])
	}
}

func TestReadyHandlerNotReady(t *testing.T) {
	h := NewHandler()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/health/ready", nil)
	h.ReadyHandler()(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "not_ready" {
		t.Errorf("status = %q", body["status"])
	}
}

func TestReadyHandlerReady(t *testing.T) {
	h := NewHandler()
	h.SetReady(true)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/health/ready", nil)
	h.ReadyHandler()(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "ready" {
		t.Errorf("status = %q", body["status"])
	}
}

func TestReadyHandlerWithCheckerPass(t *testing.T) {
	h := NewHandler()
	h.SetReady(true)
	h.Register("db", CheckerFunc(func(_ context.Context) error {
		return nil
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/health/ready", nil)
	h.ReadyHandler()(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
}

func TestReadyHandlerWithCheckerFail(t *testing.T) {
	h := NewHandler()
	h.SetReady(true)
	h.Register("db", CheckerFunc(func(_ context.Context) error {
		return errors.New("connection refused")
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/health/ready", nil)
	h.ReadyHandler()(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	failures := body["failures"].(map[string]interface{})
	if failures["db"] != "connection refused" {
		t.Errorf("failure = %v", failures)
	}
}

func TestReadyHandlerMultipleCheckers(t *testing.T) {
	h := NewHandler()
	h.SetReady(true)
	h.Register("db", CheckerFunc(func(_ context.Context) error { return nil }))
	h.Register("cache", CheckerFunc(func(_ context.Context) error { return errors.New("down") }))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/health/ready", nil)
	h.ReadyHandler()(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}

func TestSetReady(t *testing.T) {
	h := NewHandler()
	h.SetReady(true)
	h.SetReady(false)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/health/ready", nil)
	h.ReadyHandler()(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("after SetReady(false): status = %d", w.Code)
	}
}
