package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/asymmetric-effort/ginger/internal/config"
)

func TestServerConfigDecode(t *testing.T) {
	yaml := `
otlp_http_port: 4318
query_port: 16686
admin_port: 14269
max_traces: 5000
max_body_bytes: 2097152
log_level: debug
log_format: text
pprof_enabled: true
sampling_file: /etc/ginger/sampling.json
queue_size: 2048
batch_size: 256
batch_delay: 100ms
drain_timeout: 10s
`
	var cfg ServerConfig
	if err := config.Decode([]byte(yaml), &cfg); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	checks := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"OTLPHTTPPort", cfg.OTLPHTTPPort, 4318},
		{"QueryPort", cfg.QueryPort, 16686},
		{"AdminPort", cfg.AdminPort, 14269},
		{"MaxTraces", cfg.MaxTraces, 5000},
		{"MaxBodyBytes", cfg.MaxBodyBytes, int64(2097152)},
		{"LogLevel", cfg.LogLevel, "debug"},
		{"LogFormat", cfg.LogFormat, "text"},
		{"PprofEnabled", cfg.PprofEnabled, true},
		{"SamplingFile", cfg.SamplingFile, "/etc/ginger/sampling.json"},
		{"QueueSize", cfg.QueueSize, 2048},
		{"BatchSize", cfg.BatchSize, 256},
		{"BatchDelay", cfg.BatchDelay, "100ms"},
		{"DrainTimeout", cfg.DrainTimeout, "10s"},
	}

	for _, c := range checks {
		if fmt.Sprintf("%v", c.got) != fmt.Sprintf("%v", c.want) {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestServerConfigDefaults(t *testing.T) {
	cfg := defaultConfig()
	if cfg.OTLPHTTPPort != 4318 {
		t.Errorf("default OTLPHTTPPort = %d, want 4318", cfg.OTLPHTTPPort)
	}
	if cfg.QueryPort != 16686 {
		t.Errorf("default QueryPort = %d, want 16686", cfg.QueryPort)
	}
	if cfg.AdminPort != 14269 {
		t.Errorf("default AdminPort = %d, want 14269", cfg.AdminPort)
	}
	if cfg.MaxTraces != 10000 {
		t.Errorf("default MaxTraces = %d, want 10000", cfg.MaxTraces)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("default LogLevel = %q, want info", cfg.LogLevel)
	}
	if cfg.PprofEnabled {
		t.Error("default PprofEnabled should be false")
	}
}

func TestServerConfigDecodePartial(t *testing.T) {
	yaml := `
otlp_http_port: 9999
log_level: warn
`
	cfg := defaultConfig()
	if err := config.Decode([]byte(yaml), &cfg); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if cfg.OTLPHTTPPort != 9999 {
		t.Errorf("OTLPHTTPPort = %d, want 9999", cfg.OTLPHTTPPort)
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("LogLevel = %q, want warn", cfg.LogLevel)
	}
	// Defaults should be preserved for unset fields
	if cfg.QueryPort != 16686 {
		t.Errorf("QueryPort = %d, want 16686 (default)", cfg.QueryPort)
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"debug", "DEBUG"},
		{"info", "INFO"},
		{"warn", "WARN"},
		{"error", "ERROR"},
		{"unknown", "INFO"},
	}
	for _, tt := range tests {
		got := parseLogLevel(tt.input)
		if got.String() != tt.want {
			t.Errorf("parseLogLevel(%q) = %s, want %s", tt.input, got.String(), tt.want)
		}
	}
}

func TestDefaultSamplingProvider(t *testing.T) {
	p := &defaultSamplingProvider{}
	strategy, err := p.GetSamplingStrategy("test-service")
	if err != nil {
		t.Fatalf("GetSamplingStrategy failed: %v", err)
	}
	if strategy.ProbabilisticSampling == nil {
		t.Fatal("expected probabilistic sampling strategy")
	}
	if strategy.ProbabilisticSampling.SamplingRate != 1.0 {
		t.Errorf("sampling rate = %f, want 1.0", strategy.ProbabilisticSampling.SamplingRate)
	}
}

func getFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

func TestServerHealthCheck(t *testing.T) {
	cfg := defaultConfig()
	cfg.OTLPHTTPPort = getFreePort(t)
	cfg.QueryPort = getFreePort(t)
	cfg.AdminPort = getFreePort(t)
	cfg.LogLevel = "error"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- runWithContext(ctx, cfg)
	}()

	// Wait for admin server to be ready
	adminURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.AdminPort)
	var resp *http.Response
	var err error
	for i := 0; i < 50; i++ {
		resp, err = http.Get(adminURL + "/health/live")
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("admin server not reachable: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("health/live status = %d, want 200", resp.StatusCode)
	}

	// Check readiness
	resp2, err := http.Get(adminURL + "/health/ready")
	if err != nil {
		t.Fatalf("ready check failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("health/ready status = %d, want 200", resp2.StatusCode)
	}

	cancel()
}
