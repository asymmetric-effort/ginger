package config

import (
	"context"
	"os"
	"sync"
	"time"
)

// Watcher polls a config file for changes and calls the callback when it changes.
type Watcher struct {
	path     string
	interval time.Duration
	lastHash string
	mu       sync.Mutex
}

// NewWatcher creates a new config file watcher.
func NewWatcher(path string, interval time.Duration) *Watcher {
	return &Watcher{
		path:     path,
		interval: interval,
	}
}

// Watch starts polling the file. Calls onChange when the file content changes.
// Blocks until the context is cancelled.
func (w *Watcher) Watch(ctx context.Context, onChange func(data []byte)) {
	// Read initial content
	if data, err := os.ReadFile(w.path); err == nil {
		w.mu.Lock()
		w.lastHash = hashContent(data)
		w.mu.Unlock()
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.check(onChange)
		}
	}
}

func (w *Watcher) check(onChange func(data []byte)) {
	data, err := os.ReadFile(w.path)
	if err != nil {
		return
	}

	h := hashContent(data)
	w.mu.Lock()
	changed := h != w.lastHash
	if changed {
		w.lastHash = h
	}
	w.mu.Unlock()

	if changed {
		onChange(data)
	}
}

// hashContent creates a simple hash of the content for change detection.
// Uses a basic FNV-1a hash — no crypto needed here.
func hashContent(data []byte) string {
	var h uint64 = 14695981039346656037 // FNV offset basis
	for _, b := range data {
		h ^= uint64(b)
		h *= 1099511628211 // FNV prime
	}
	buf := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		buf[i] = byte(h & 0xff)
		h >>= 8
	}
	const hexDigits = "0123456789abcdef"
	out := make([]byte, 16)
	for i, b := range buf {
		out[i*2] = hexDigits[b>>4]
		out[i*2+1] = hexDigits[b&0x0f]
	}
	return string(out)
}
