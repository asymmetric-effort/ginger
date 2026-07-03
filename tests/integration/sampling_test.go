package integration

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/asymmetric-effort/ginger/internal/sampling"
)

func TestRemoteSamplingHTTP(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "strategies.json")
	os.WriteFile(path, []byte(`{
		"service_strategies": [
			{"service": "frontend", "type": "probabilistic", "param": 0.5}
		],
		"default_strategy": {"strategyType": "PROBABILISTIC", "probabilisticSampling": {"samplingRate": 0.01}}
	}`), 0644)

	fp, err := sampling.NewFileProvider(path)
	if err != nil {
		t.Fatal(err)
	}
	handler := sampling.NewHandler(fp)

	// Test known service
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/sampling?service=frontend", nil)
	handler.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Errorf("status = %d", w.Code)
	}
	var strategy sampling.Strategy
	json.Unmarshal(w.Body.Bytes(), &strategy)
	if strategy.ProbabilisticSampling.SamplingRate != 0.5 {
		t.Errorf("rate = %f", strategy.ProbabilisticSampling.SamplingRate)
	}

	// Test unknown service (returns default)
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/sampling?service=unknown", nil)
	handler.ServeHTTP(w, r)
	json.Unmarshal(w.Body.Bytes(), &strategy)
	if strategy.ProbabilisticSampling.SamplingRate != 0.01 {
		t.Errorf("default rate = %f", strategy.ProbabilisticSampling.SamplingRate)
	}
}

func TestFileBasedStrategyReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "strategies.json")
	os.WriteFile(path, []byte(`{
		"service_strategies": [{"service": "svc", "type": "probabilistic", "param": 0.1}]
	}`), 0644)

	fp, _ := sampling.NewFileProvider(path)
	s, _ := fp.GetSamplingStrategy("svc")
	if s.ProbabilisticSampling.SamplingRate != 0.1 {
		t.Error("initial rate")
	}

	// Update file
	os.WriteFile(path, []byte(`{
		"service_strategies": [{"service": "svc", "type": "probabilistic", "param": 0.9}]
	}`), 0644)
	fp.Reload()

	s, _ = fp.GetSamplingStrategy("svc")
	if s.ProbabilisticSampling.SamplingRate != 0.9 {
		t.Error("reloaded rate")
	}
}
