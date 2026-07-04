package influxdb

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/asymmetric-effort/ginger/internal/codec/influxlp"
)

// ClientConfig configures the InfluxDB HTTP client.
type ClientConfig struct {
	Endpoint      string
	Org           string
	Bucket        string
	Token         string
	TLSConfig     *tls.Config
	WriteTimeout  time.Duration
	QueryTimeout  time.Duration
	MaxBatchSize  int
	MaxBatchBytes int64
}

// Client communicates with InfluxDB v2 API.
type Client struct {
	config     ClientConfig
	httpClient *http.Client
	encoder    *influxlp.Encoder

	// Circuit breaker
	failures     atomic.Int64
	circuitOpen  atomic.Bool
	circuitReset time.Time
	circuitMu    sync.Mutex
}

// NewClient creates a new InfluxDB client.
func NewClient(config ClientConfig) *Client {
	if config.WriteTimeout == 0 {
		config.WriteTimeout = 10 * time.Second
	}
	if config.QueryTimeout == 0 {
		config.QueryTimeout = 30 * time.Second
	}
	if config.MaxBatchSize == 0 {
		config.MaxBatchSize = 5000
	}
	if config.MaxBatchBytes == 0 {
		config.MaxBatchBytes = 5 * 1024 * 1024
	}

	transport := &http.Transport{
		TLSClientConfig:     config.TLSConfig,
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	return &Client{
		config: config,
		httpClient: &http.Client{
			Transport: transport,
		},
		encoder: influxlp.NewEncoder(),
	}
}

// Write sends data points to InfluxDB.
func (c *Client) Write(ctx context.Context, points []influxlp.Point) error {
	if c.isCircuitOpen() {
		return ErrCircuitOpen
	}

	data, err := c.encoder.EncodeMulti(points)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	url := c.config.Endpoint + "/api/v2/write?org=" + c.config.Org + "&bucket=" + c.config.Bucket + "&precision=ns"

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		writeCtx, cancel := context.WithTimeout(ctx, c.config.WriteTimeout)
		req, err := http.NewRequestWithContext(writeCtx, http.MethodPost, url, strings.NewReader(string(data)))
		if err != nil {
			cancel()
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "text/plain; charset=utf-8")
		if c.config.Token != "" {
			req.Header.Set("Authorization", "Token "+c.config.Token)
		}

		resp, err := c.httpClient.Do(req)
		cancel()
		if err != nil {
			lastErr = err
			c.recordFailure()
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			c.recordSuccess()
			return nil
		}

		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
			// Client error (not rate limit) — don't retry
			c.recordSuccess() // not a server failure
			return fmt.Errorf("write failed: HTTP %d", resp.StatusCode)
		}

		// 429 or 5xx — retry
		lastErr = fmt.Errorf("write failed: HTTP %d", resp.StatusCode)
		c.recordFailure()
	}

	return lastErr
}

// FluxRecord represents a row from a Flux query result.
type FluxRecord struct {
	Values map[string]string
}

// Query executes a Flux query and returns the results.
func (c *Client) Query(ctx context.Context, flux string) ([]FluxRecord, error) {
	url := c.config.Endpoint + "/api/v2/query?org=" + c.config.Org

	queryCtx, cancel := context.WithTimeout(ctx, c.config.QueryTimeout)
	defer cancel()

	body := `{"query": "` + escapeJSON(flux) + `", "type": "flux"}`
	req, err := http.NewRequestWithContext(queryCtx, http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/csv")
	if c.config.Token != "" {
		req.Header.Set("Authorization", "Token "+c.config.Token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("query failed: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return parseCSV(string(respBody))
}

// Close closes the client and releases resources.
func (c *Client) Close() {
	c.httpClient.CloseIdleConnections()
}

func (c *Client) isCircuitOpen() bool {
	if !c.circuitOpen.Load() {
		return false
	}
	c.circuitMu.Lock()
	defer c.circuitMu.Unlock()
	if time.Now().After(c.circuitReset) {
		c.circuitOpen.Store(false)
		c.failures.Store(0)
		return false
	}
	return true
}

func (c *Client) recordFailure() {
	count := c.failures.Add(1)
	if count >= 5 {
		c.circuitMu.Lock()
		c.circuitOpen.Store(true)
		c.circuitReset = time.Now().Add(30 * time.Second)
		c.circuitMu.Unlock()
	}
}

func (c *Client) recordSuccess() {
	c.failures.Store(0)
	c.circuitOpen.Store(false)
}

func parseCSV(data string) ([]FluxRecord, error) {
	lines := strings.Split(strings.TrimSpace(data), "\n")
	if len(lines) < 2 {
		return nil, nil
	}

	// First line with content starting with "#" is annotation, skip it
	headerIdx := 0
	for headerIdx < len(lines) {
		if !strings.HasPrefix(lines[headerIdx], "#") && lines[headerIdx] != "" {
			break
		}
		headerIdx++
	}
	if headerIdx >= len(lines) {
		return nil, nil
	}

	headers := strings.Split(lines[headerIdx], ",")
	var records []FluxRecord
	for i := headerIdx + 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		fields := strings.Split(line, ",")
		record := FluxRecord{Values: make(map[string]string)}
		for j, h := range headers {
			if j < len(fields) {
				record.Values[strings.TrimSpace(h)] = strings.TrimSpace(fields[j])
			}
		}
		records = append(records, record)
	}
	return records, nil
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

var (
	// ErrCircuitOpen is returned when the circuit breaker is open.
	ErrCircuitOpen = errors.New("influxdb: circuit breaker open")
)
