package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"lists/internal/auth"
	"lists/internal/db"
)

const (
	maxTextLen = 500
	// maxPinned is how many lists a user can pin to the home page.
	maxPinned = 5
)

// listLists returns every top-level list.
func (s *Server) listLists(w http.ResponseWriter, r *http.Request) {
	rows, err := s.q.ListTopLevel(r.Context(), auth.UserID(r.Context()))
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

// autocompleteLists returns the best few list-title matches for ?q=.
func (s *Server) autocompleteLists(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" || len([]rune(q)) > maxTextLen {
		writeJSON(w, http.StatusOK, []db.AutocompleteListsRow{})
		return
	}
	// The query is matched literally, so neutralise LIKE wildcards.
	q = likeEscaper.Replace(q)
	rows, err := s.q.AutocompleteLists(r.Context(), db.AutocompleteListsParams{
		UserID: auth.UserID(r.Context()), Query: q,
	})
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func (s *Server) recentLists(w http.ResponseWriter, r *http.Request) {
	rows, err := s.q.ListRecent(r.Context(), auth.UserID(r.Context()))
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *Server) createList(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title string `json:"title"`
	}
	text, ok := decodeText(w, r, &body, &body.Title)
	if !ok {
		return
	}
	item, err := s.q.CreateItem(r.Context(), db.CreateItemParams{
		UserID: auth.UserID(r.Context()), Text: text, IsList: true,
	})
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) getList(w http.ResponseWriter, r *http.Request) {
	user := auth.UserID(r.Context())
	list, ok := s.loadList(w, r, user)
	if !ok {
		return
	}
	entries, err := s.q.ListEntries(r.Context(), db.ListEntriesParams{ParentID: &list.ID, UserID: user})
	if err != nil {
		serverError(w, err)
		return
	}
	ancestors, err := s.q.ListAncestors(r.Context(), db.ListAncestorsParams{ID: list.ID, UserID: user})
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"list":      list,
		"entries":   entries,
		"ancestors": ancestors,
	})
}

func (s *Server) createEntry(w http.ResponseWriter, r *http.Request) {
	user := auth.UserID(r.Context())
	list, ok := s.loadList(w, r, user)
	if !ok {
		return
	}
	var body struct {
		Text   string `json:"text"`
		IsList bool   `json:"is_list"`
	}
	text, ok := decodeText(w, r, &body, &body.Text)
	if !ok {
		return
	}

	var item db.ListsItem
	err := s.inTx(r.Context(), func(q *db.Queries) error {
		var err error
		item, err = q.CreateItem(r.Context(), db.CreateItemParams{
			UserID: user, ParentID: &list.ID, Text: text, IsList: body.IsList,
		})
		if err != nil {
			return err
		}
		return q.TouchWithAncestors(r.Context(), list.ID)
	})
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// reorderEntries sets the order of a list's entries to the given id order.
// Ids that are not entries of this list are ignored.
func (s *Server) reorderEntries(w http.ResponseWriter, r *http.Request) {
	user := auth.UserID(r.Context())
	list, ok := s.loadList(w, r, user)
	if !ok {
		return
	}
	var body struct {
		IDs []uuid.UUID `json:"ids"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(body.IDs) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "ids is required")
		return
	}

	err := s.inTx(r.Context(), func(q *db.Queries) error {
		if err := q.ReorderEntries(r.Context(), db.ReorderEntriesParams{
			ParentID: &list.ID, UserID: user, Ids: body.IDs,
		}); err != nil {
			return err
		}
		return q.TouchWithAncestors(r.Context(), list.ID)
	})
	if err != nil {
		serverError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// setPinned pins a top-level list to the home page, or unpins it.
func (s *Server) setPinned(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Pinned *bool `json:"pinned"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil || body.Pinned == nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	user := auth.UserID(r.Context())
	if *body.Pinned {
		n, err := s.q.CountPinned(r.Context(), db.CountPinnedParams{UserID: user, ID: id})
		if err != nil {
			serverError(w, err)
			return
		}
		if n >= maxPinned {
			writeError(w, http.StatusConflict, fmt.Sprintf("You can pin at most %d lists. Unpin one first.", maxPinned))
			return
		}
	}
	item, err := s.q.SetPinned(r.Context(), db.SetPinnedParams{ID: id, UserID: user, Pinned: *body.Pinned})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "list not found")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// setCompleted marks an entry completed, or reopens it.
func (s *Server) setCompleted(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Completed *bool `json:"completed"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil || body.Completed == nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var item db.ListsItem
	err := s.inTx(r.Context(), func(q *db.Queries) error {
		var err error
		item, err = q.SetCompleted(r.Context(), db.SetCompletedParams{
			ID: id, UserID: auth.UserID(r.Context()), Completed: *body.Completed,
		})
		if err != nil {
			return err
		}
		return q.TouchWithAncestors(r.Context(), id)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "entry not found")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// updateItem changes an item's text, turns an entry into a list, or both.
// A list cannot be turned back into an entry: its contents would be orphaned.
func (s *Server) updateItem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Text   *string `json:"text"`
		IsList *bool   `json:"is_list"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	makeList := body.IsList != nil && *body.IsList
	if body.IsList != nil && !makeList {
		writeError(w, http.StatusUnprocessableEntity, "a list cannot be turned back into an entry")
		return
	}
	if body.Text == nil && !makeList {
		writeError(w, http.StatusUnprocessableEntity, "nothing to update")
		return
	}
	var text string
	if body.Text != nil {
		if text, ok = validText(w, *body.Text); !ok {
			return
		}
	}

	user := auth.UserID(r.Context())
	var item db.ListsItem
	err := s.inTx(r.Context(), func(q *db.Queries) error {
		var err error
		if body.Text != nil {
			if item, err = q.UpdateItemText(r.Context(), db.UpdateItemTextParams{ID: id, UserID: user, Text: text}); err != nil {
				return err
			}
		}
		if makeList {
			if item, err = q.MakeItemList(r.Context(), db.MakeItemListParams{ID: id, UserID: user}); err != nil {
				return err
			}
		}
		return q.TouchWithAncestors(r.Context(), id)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) deleteItem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	user := auth.UserID(r.Context())

	var deleted int64
	err := s.inTx(r.Context(), func(q *db.Queries) error {
		item, err := q.GetItem(r.Context(), db.GetItemParams{ID: id, UserID: user})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if deleted, err = q.DeleteItem(r.Context(), db.DeleteItemParams{ID: id, UserID: user}); err != nil {
			return err
		}
		if item.ParentID != nil {
			return q.TouchWithAncestors(r.Context(), *item.ParentID)
		}
		return nil
	})
	if err != nil {
		serverError(w, err)
		return
	}
	if deleted == 0 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// loadList fetches the {id} item for the user and rejects plain entries.
func (s *Server) loadList(w http.ResponseWriter, r *http.Request, user uuid.UUID) (db.ListsItem, bool) {
	id, ok := pathID(w, r)
	if !ok {
		return db.ListsItem{}, false
	}
	item, err := s.q.GetItem(r.Context(), db.GetItemParams{ID: id, UserID: user})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !item.IsList) {
		writeError(w, http.StatusNotFound, "list not found")
		return db.ListsItem{}, false
	}
	if err != nil {
		serverError(w, err)
		return db.ListsItem{}, false
	}
	return item, true
}

func (s *Server) inTx(ctx context.Context, fn func(q *db.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(s.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func pathID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return uuid.Nil, false
	}
	return id, true
}

// decodeText reads the JSON body into dst and validates the one text field.
func decodeText(w http.ResponseWriter, r *http.Request, dst any, field *string) (string, bool) {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return "", false
	}
	return validText(w, *field)
}

func validText(w http.ResponseWriter, raw string) (string, bool) {
	text := strings.TrimSpace(raw)
	if text == "" {
		writeError(w, http.StatusUnprocessableEntity, "text is required")
		return "", false
	}
	if len([]rune(text)) > maxTextLen {
		writeError(w, http.StatusUnprocessableEntity, "text is too long")
		return "", false
	}
	return text, true
}

func serverError(w http.ResponseWriter, err error) {
	log.Printf("internal error: %v", err)
	writeError(w, http.StatusInternalServerError, "internal server error")
}
