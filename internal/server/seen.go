package server

import (
	"encoding/json"
	"net/http"
	"sort"
	"sync"
	"time"
)

// modelHit records one model ID the client asked for.
type modelHit struct {
	Requested string    `json:"requested"`
	Mapped    string    `json:"mapped"`
	Route     string    `json:"route"` // "v1beta" or "v1internal"
	Count     int       `json:"count"`
	LastSeen  time.Time `json:"last_seen"`
}

// registry remembers every model ID Antigravity has asked for, so operators can
// build a MODEL_MAP from real traffic instead of guessing.
type registry struct {
	mu   sync.Mutex
	hits map[string]*modelHit
}

func newRegistry() *registry {
	return &registry{hits: map[string]*modelHit{}}
}

func (r *registry) record(requested, mapped, route string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	h, ok := r.hits[requested]
	if !ok {
		h = &modelHit{Requested: requested, Mapped: mapped, Route: route}
		r.hits[requested] = h
	}
	h.Count++
	h.LastSeen = time.Now()
	h.Mapped = mapped
}

func (r *registry) snapshot() []modelHit {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]modelHit, 0, len(r.hits))
	for _, h := range r.hits {
		out = append(out, *h)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Route != out[j].Route {
			return out[i].Route < out[j].Route
		}
		return out[i].Requested < out[j].Requested
	})
	return out
}

// handleSeen reports the observed model IDs.
func (s *Server) handleSeen(w http.ResponseWriter, r *http.Request) {
	seen := s.seen.snapshot()
	ids := make([]string, 0, len(seen))
	for _, h := range seen {
		ids = append(ids, h.Requested)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"count":  len(seen),
		"models": ids,
		"detail": seen,
	})
}
