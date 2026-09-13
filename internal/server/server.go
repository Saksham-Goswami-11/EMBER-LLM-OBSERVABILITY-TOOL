// Package server wires the OTLP receiver, the query API, and the embedded
// dashboard into one http.Handler — the whole product, one port.
package server

import (
	"io/fs"
	"log"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/sakshamgoswami/ember/internal/api"
	"github.com/sakshamgoswami/ember/internal/ingest"
	"github.com/sakshamgoswami/ember/internal/pricing"
	"github.com/sakshamgoswami/ember/internal/storage"
)

func New(store *storage.Store, prices *pricing.Table, webFS fs.FS) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/v1/traces", &ingest.Handler{Store: store, Pricing: prices})
	mux.Handle("/api/", api.New(store))
	mux.HandleFunc("/healthz", handleHealth)
	mux.Handle("/", spaHandler(webFS))
	return withLogging(mux)
}

// handleHealth is what the container healthcheck hits. It deliberately
// reports nothing about the instance beyond liveness — it's reachable
// without an API key.
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// spaHandler serves the built dashboard, falling back to index.html for
// any path that isn't a real asset so client-side routing works — or, if
// the frontend hasn't been built yet, a plain-text nudge instead of a bare
// 404.
func spaHandler(webFS fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(webFS))
	_, indexErr := fs.Stat(webFS, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if indexErr != nil {
			http.Error(w, "Ember's dashboard hasn't been built yet.\n\n"+
				"Run:\n  cd web && npm install && npm run build\n\nthen restart ember.",
				http.StatusServiceUnavailable)
			return
		}
		clean := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if clean == "" {
			clean = "."
		}
		if _, err := fs.Stat(webFS, clean); err != nil {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Millisecond))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
