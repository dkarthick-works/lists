// Package api wires the HTTP routes.
package api

import (
	"encoding/json"
	"io/fs"
	"mime"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"lists/internal/auth"
	"lists/internal/db"
	"lists/web"
)

type Server struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewRouter(pool *pgxpool.Pool, verifier *auth.Verifier, authProxy http.Handler) http.Handler {
	s := &Server{pool: pool, q: db.New(pool)}

	r := chi.NewRouter()
	r.Use(middleware.Logger, middleware.Recoverer)

	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Handle("/api/auth/*", authProxy)

	r.Route("/api", func(r chi.Router) {
		// Per-user data: keep it out of every HTTP cache.
		r.Use(middleware.SetHeader("Cache-Control", "no-store"))
		r.Use(verifier.Middleware)
		r.Get("/lists", s.listLists)
		r.Post("/lists", s.createList)
		r.Get("/lists/recent", s.recentLists)
		r.Get("/lists/autocomplete", s.autocompleteLists)
		r.Get("/lists/{id}", s.getList)
		r.Post("/lists/{id}/entries", s.createEntry)
		r.Patch("/items/{id}", s.updateItem)
		r.Delete("/items/{id}", s.deleteItem)
	})

	if web.FS != nil {
		r.NotFound(spaHandler(web.FS).ServeHTTP)
	}
	return r
}

// spaHandler serves the embedded frontend, falling back to index.html for
// client-side routes.
func spaHandler(fsys fs.FS) http.Handler {
	// Not in Go's built-in table, and the runtime image ships no mime.types.
	mime.AddExtensionType(".webmanifest", "application/manifest+json")
	fserver := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		if f, err := fsys.Open(name); err != nil {
			name = "index.html"
			r.URL.Path = "/"
		} else {
			f.Close()
		}

		// HTML and service-worker entry points must be revalidated so a new
		// deployment is picked up immediately. Vite's content-hashed assets are
		// immutable and safe to cache for a year.
		switch {
		case name == "index.html", name == "sw.js", name == "registerSW.js":
			w.Header().Set("Cache-Control", "no-cache")
		case strings.HasPrefix(name, "assets/"):
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		fserver.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
