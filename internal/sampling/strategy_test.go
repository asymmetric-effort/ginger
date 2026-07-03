package sampling

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileProviderBasic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "strategies.json")
	data := `{
		"default_strategy": {"strategyType": "PROBABILISTIC", "probabilisticSampling": {"samplingRate": 0.01}},
		"service_strategies": [
			{"service": "frontend", "type": "probabilistic", "param": 0.5},
			{"service": "backend", "type": "rate_limiting", "param": 10}
		]
	}`
	os.WriteFile(path, []byte(data), 0644)

	fp, err := NewFileProvider(path)
	if err != nil {
		t.Fatal(err)
	}

	s, err := fp.GetSamplingStrategy("frontend")
	if err != nil {
		t.Fatal(err)
	}
	if s.Type != StrategyProbabilistic || s.ProbabilisticSampling.SamplingRate != 0.5 {
		t.Errorf("frontend: %+v", s)
	}

	s, err = fp.GetSamplingStrategy("backend")
	if err != nil {
		t.Fatal(err)
	}
	if s.Type != StrategyRateLimiting || s.RateLimitingSampling.MaxTracesPerSecond != 10 {
		t.Errorf("backend: %+v", s)
	}
}

func TestFileProviderDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "strategies.json")
	os.WriteFile(path, []byte(`{
		"default_strategy": {"strategyType": "PROBABILISTIC", "probabilisticSampling": {"samplingRate": 0.01}},
		"service_strategies": []
	}`), 0644)

	fp, _ := NewFileProvider(path)
	s, _ := fp.GetSamplingStrategy("unknown")
	if s.ProbabilisticSampling.SamplingRate != 0.01 {
		t.Error("should return default strategy")
	}
}

func TestFileProviderNoDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "strategies.json")
	os.WriteFile(path, []byte(`{"service_strategies": []}`), 0644)

	fp, _ := NewFileProvider(path)
	s, _ := fp.GetSamplingStrategy("unknown")
	if s.ProbabilisticSampling.SamplingRate != 1.0 {
		t.Error("should return 100% sampling as fallback")
	}
}

func TestFileProviderInvalidFile(t *testing.T) {
	_, err := NewFileProvider("/nonexistent")
	if err == nil {
		t.Error("should error for nonexistent file")
	}
}

func TestFileProviderInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("not json"), 0644)

	_, err := NewFileProvider(path)
	if err == nil {
		t.Error("should error for invalid JSON")
	}
}

func TestFileProviderReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "strategies.json")
	os.WriteFile(path, []byte(`{
		"service_strategies": [{"service": "svc", "type": "probabilistic", "param": 0.1}]
	}`), 0644)

	fp, _ := NewFileProvider(path)
	s, _ := fp.GetSamplingStrategy("svc")
	if s.ProbabilisticSampling.SamplingRate != 0.1 {
		t.Error("initial rate")
	}

	os.WriteFile(path, []byte(`{
		"service_strategies": [{"service": "svc", "type": "probabilistic", "param": 0.9}]
	}`), 0644)
	fp.Reload()

	s, _ = fp.GetSamplingStrategy("svc")
	if s.ProbabilisticSampling.SamplingRate != 0.9 {
		t.Error("reloaded rate")
	}
}

func TestFileProviderWatchFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "strategies.json")
	os.WriteFile(path, []byte(`{"service_strategies": []}`), 0644)

	fp, _ := NewFileProvider(path)
	done := make(chan struct{})
	go fp.WatchFile(50*time.Millisecond, done)
	time.Sleep(150 * time.Millisecond)
	close(done)
}

func TestHandlerSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "strategies.json")
	os.WriteFile(path, []byte(`{
		"service_strategies": [{"service": "svc", "type": "probabilistic", "param": 0.5}]
	}`), 0644)

	fp, _ := NewFileProvider(path)
	h := NewHandler(fp)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/sampling?service=svc", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
	var s Strategy
	json.Unmarshal(w.Body.Bytes(), &s)
	if s.ProbabilisticSampling.SamplingRate != 0.5 {
		t.Error("rate")
	}
}

func TestHandlerMissingService(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "strategies.json")
	os.WriteFile(path, []byte(`{"service_strategies": []}`), 0644)

	fp, _ := NewFileProvider(path)
	h := NewHandler(fp)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/sampling", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}
}

// errorProvider always returns an error from GetSamplingStrategy.
type errorProvider struct{}

func (e *errorProvider) GetSamplingStrategy(_ string) (*Strategy, error) {
	return nil, errProviderFailed
}

var errProviderFailed = fmt.Errorf("provider failure")

func TestHandlerProviderError(t *testing.T) {
	h := NewHandler(&errorProvider{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/sampling?service=svc", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}
