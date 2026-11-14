// Package server exposes the HTTP surface that Antigravity talks to.
package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"agy-bridge/internal/config"
)

// Server holds shared state for every handler.
type Server struct {
	Cfg    config.Config
	Client *http.Client
	seen   *registry
}

// New builds a Server. The HTTP client has no overall timeout because a single
// agent turn can legitimately run for minutes.
func New(cfg config.Config) *Server {
	return &Server{Cfg: cfg, Client: &http.Client{Timeout: 0}, seen: newRegistry()}
}

// Handler returns the root mux.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/seen", s.handleSeen)
	mux.HandleFunc("/", s.handle)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":            true,
		"upstream":      s.Cfg.UpstreamBase,
		"default_model": s.Cfg.DefaultModel,
		"model_map":     s.Cfg.ModelMap,
	})
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Cloud Code RPCs that use a slash instead of a colon, e.g.
	// GET /v1internal/cascadeNuxes. These are account/UI calls: forward them.
	if i := strings.Index(path, "/v1internal/"); i >= 0 {
		raw, _ := readAll(r)
		s.forwardToGoogle(w, r, raw)
		return
	}

	// Cloud Code API, used by the Antigravity IDE and its hub.
	if i := strings.Index(path, "v1internal:"); i >= 0 {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "POST required", "INVALID_ARGUMENT")
			return
		}
		raw, err := readAll(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "read body: "+err.Error(), "INVALID_ARGUMENT")
			return
		}
		s.handleCloud(w, r, path[i+len("v1internal:"):], raw)
		return
	}

	if strings.HasSuffix(path, "/models") || strings.HasSuffix(path, "/models/") {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"models": []any{}})
		return
	}

	s.handleGemini(w, r)
}

// writeError emits a Gemini-shaped error envelope.
func writeError(w http.ResponseWriter, code int, msg, status string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{"code": code, "message": msg, "status": status},
	})
}

// writeJSON emits a 200 JSON body.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// callerKey extracts the upstream credential presented by the caller.
func callerKey(r *http.Request) string {
	if k := r.Header.Get("X-Goog-Api-Key"); k != "" {
		return k
	}
	if k := r.URL.Query().Get("key"); k != "" {
		return k
	}
	if a := r.Header.Get("Authorization"); strings.HasPrefix(a, "Bearer ") {
		return strings.TrimPrefix(a, "Bearer ")
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func logf(format string, args ...any) {
	log.Printf(format, args...)
}

var _ = time.Now
