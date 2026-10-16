package api

import (
	"net/http"
	"strings"

	"lists/internal/auth"
	"lists/internal/db"
)

const maxKeyNameLen = 100

func (s *Server) listKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := s.q.ListAPIKeys(r.Context(), auth.UserID(r.Context()))
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, keys)
}

// createKey answers with the raw key, which is never shown again.
func (s *Server) createKey(w http.ResponseWriter, r *http.Request) {
	body, ok := decode[struct {
		Name string `json:"name"`
	}](w, r)
	if !ok {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeError(w, http.StatusUnprocessableEntity, "name is required")
		return
	}
	if len([]rune(name)) > maxKeyNameLen {
		writeError(w, http.StatusUnprocessableEntity, "name is too long")
		return
	}
	key, hash := auth.NewKey()
	row, err := s.q.CreateAPIKey(r.Context(), db.CreateAPIKeyParams{UserID: auth.UserID(r.Context()), Name: name, KeyHash: hash})
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, struct {
		db.CreateAPIKeyRow
		Key string `json:"key"`
	}{row, key})
}

func (s *Server) revokeKey(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	n, err := s.q.RevokeAPIKey(r.Context(), db.RevokeAPIKeyParams{ID: id, UserID: auth.UserID(r.Context())})
	if err != nil {
		serverError(w, err)
		return
	}
	if n == 0 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
