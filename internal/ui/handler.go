package ui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dist
var uiFS embed.FS

// Handler serves the embedded UI static files as a SPA.
type Handler struct {
	fileServer http.Handler
}

// NewHandler creates a UI handler.
func NewHandler() (*Handler, error) {
	sub, err := fs.Sub(uiFS, "dist")
	if err != nil {
		return nil, err
	}
	return &Handler{
		fileServer: http.FileServer(http.FS(sub)),
	}, nil
}

// ServeHTTP serves static files, falling back to index.html for SPA routing.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// API routes are not handled by the UI
	if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/v1/") ||
		strings.HasPrefix(path, "/health/") || strings.HasPrefix(path, "/debug/") ||
		strings.HasPrefix(path, "/sampling") {
		http.NotFound(w, r)
		return
	}

	// Try to serve the file directly
	h.fileServer.ServeHTTP(w, r)
}
