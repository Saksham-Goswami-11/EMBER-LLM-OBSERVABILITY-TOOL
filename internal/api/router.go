// Package api exposes the read-only REST API the dashboard is built on:
// project, trace, session, and usage-analytics queries over the SQLite
// store. It has no authentication in v1 — see the README's security note
// before exposing it beyond localhost.
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/sakshamgoswami/ember/internal/model"
	"github.com/sakshamgoswami/ember/internal/storage"
)

func New(store *storage.Store) http.Handler {
	mux := http.NewServeMux()
	h := &handlers{store: store}

	mux.HandleFunc("GET /api/projects", h.listProjects)
	mux.HandleFunc("GET /api/traces", h.listTraces)
	mux.HandleFunc("GET /api/traces/{id}", h.getTrace)
	mux.HandleFunc("GET /api/sessions", h.listSessions)
	mux.HandleFunc("GET /api/analytics/usage", h.usageAnalytics)

	return mux
}

type handlers struct {
	store *storage.Store
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func projectOrDefault(r *http.Request) string {
	if p := r.URL.Query().Get("project"); p != "" {
		return p
	}
	return "default"
}

func intParam(r *http.Request, name string, def int) int {
	v := r.URL.Query().Get(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func (h *handlers) listProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.store.ListProjects(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list projects")
		return
	}
	writeJSON(w, http.StatusOK, projects)
}

func (h *handlers) listTraces(w http.ResponseWriter, r *http.Request) {
	limit := intParam(r, "limit", 100)
	before := int64(intParam(r, "before", 0))
	traces, err := h.store.ListTraces(r.Context(), projectOrDefault(r), limit, before)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list traces")
		return
	}
	if traces == nil {
		traces = []model.Trace{}
	}
	writeJSON(w, http.StatusOK, traces)
}

func (h *handlers) getTrace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	trace, spans, err := h.store.GetTrace(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to load trace")
		return
	}
	if trace == nil {
		writeErr(w, http.StatusNotFound, "trace not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"trace": trace, "spans": spans})
}

func (h *handlers) listSessions(w http.ResponseWriter, r *http.Request) {
	limit := intParam(r, "limit", 100)
	sessions, err := h.store.ListSessions(r.Context(), projectOrDefault(r), limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list sessions")
		return
	}
	if sessions == nil {
		sessions = []model.Session{}
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (h *handlers) usageAnalytics(w http.ResponseWriter, r *http.Request) {
	days := intParam(r, "days", 7)
	usage, err := h.store.UsageAnalytics(r.Context(), projectOrDefault(r), days)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to compute usage analytics")
		return
	}
	if usage == nil {
		usage = []model.UsageRow{}
	}
	writeJSON(w, http.StatusOK, usage)
}
