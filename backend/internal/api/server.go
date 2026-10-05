// Package api wires the HTTP routes.
package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"lists/internal/auth"
	"lists/internal/db"
)

type Server struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewRouter(pool *pgxpool.Pool, verifier *auth.Verifier, authProxy http.Handler, staticDir string) http.Handler {
	s := &Server{pool: pool, q: db.New(pool)}

	r := chi.NewRouter()
	r.Use(middleware.Logger, middleware.Recoverer)

	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Handle("/api/auth/*", authProxy)

	r.Route("/api", func(r chi.Router) {
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

	if staticDir != "" {
		r.NotFound(spa(staticDir))
	}
	return r
}

// spa serves the built frontend, falling back to index.html for client routes.
func spa(dir string) http.HandlerFunc {
	files := http.FileServer(http.Dir(dir))
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		if info, err := os.Stat(filepath.Join(dir, filepath.Clean("/"+r.URL.Path))); err != nil || info.IsDir() {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		files.ServeHTTP(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
