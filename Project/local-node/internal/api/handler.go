package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/potbuddy/local-node/internal/processor"
	"github.com/potbuddy/local-node/internal/store"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handler wraps the ring buffer and exposes HTTP endpoints.
type Handler struct {
	store *store.RingBuffer
}

// NewHandler creates the HTTP handler with all routes registered.
func NewHandler(rb *store.RingBuffer) http.Handler {
	h := &Handler{store: rb}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	r.Get("/health", h.healthCheck)
	r.Get("/status", h.latestStatus)
	r.Get("/history", h.history)
	r.Get("/metrics", promhttp.Handler().ServeHTTP) // Prometheus scrape endpoint

	return r
}

// corsMiddleware adds permissive CORS headers so the dashboard can call the API
// from a different origin (e.g. cloud-hosted frontend querying local node).
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// GET /health — liveness probe.
func (h *Handler) healthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GET /status — returns the latest enriched payload.
func (h *Handler) latestStatus(w http.ResponseWriter, r *http.Request) {
	latest, ok := h.store.Latest()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "no data yet — waiting for first sensor reading",
		})
		return
	}
	writeJSON(w, http.StatusOK, latest)
}

// GET /history?n=20 — returns the last N enriched payloads (default 20, max 100).
func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	n := 20
	if qs := r.URL.Query().Get("n"); qs != "" {
		v, err := strconv.Atoi(qs)
		if err != nil || v < 1 {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("invalid n=%q — must be a positive integer", qs),
			})
			return
		}
		n = v
	}
	if n > 100 {
		n = 100
	}
	readings := h.store.History(n)
	if readings == nil {
		readings = []processor.EnrichedPayload{}
	}
	writeJSON(w, http.StatusOK, readings)
}
