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
	"lists/internal/titler"
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

// createList makes a top-level list. Long text becomes a short title, with
// the full text kept as a page inside the new list.
func (s *Server) createList(w http.ResponseWriter, r *http.Request) {
	body, ok := decode[struct {
		Title string `json:"title"`
	}](w, r)
	if !ok {
		return
	}
	title, page, ok := s.titleAndPage(w, r, body.Title)
	if !ok {
		return
	}
	var list db.ListsItem
	err := s.inTx(r.Context(), func(q *db.Queries) error {
		var err error
		list, err = createList(r.Context(), q, auth.UserID(r.Context()), nil, title, page)
		return err
	})
	if err != nil {
		serverError(w, err)
		return
	}
	if page != nil {
		s.retitle(list.UserID, list.ID, title, *page)
	}
	writeJSON(w, http.StatusCreated, list)
}

// createList inserts a list and, when page is set, a page inside it.
func createList(ctx context.Context, q *db.Queries, user uuid.UUID, parent *uuid.UUID, title string, page *string) (db.ListsItem, error) {
	list, err := q.CreateItem(ctx, db.CreateItemParams{UserID: user, ParentID: parent, Text: title, IsList: true})
	if err != nil || page == nil {
		return list, err
	}
	_, err = q.CreateItem(ctx, db.CreateItemParams{UserID: user, ParentID: &list.ID, Text: title, Body: page})
	return list, err
}

// titleAndPage validates new text and decides what it becomes. Short text is
// used as typed (page is nil). Text over the word threshold, or too long for
// a title, becomes a page: the full text, titled with its opening words until
// retitle replaces that.
func (s *Server) titleAndPage(w http.ResponseWriter, r *http.Request, raw string) (title string, page *string, ok bool) {
	text := strings.TrimSpace(raw)
	if text == "" {
		writeError(w, http.StatusUnprocessableEntity, "text is required")
		return "", nil, false
	}
	runes := len([]rune(text))
	if len(strings.Fields(text)) <= s.pages.WordThreshold && runes <= maxTextLen {
		return text, nil, true
	}
	if runes > s.pages.MaxChars {
		writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("text is too long (limit %d characters)", s.pages.MaxChars))
		return "", nil, false
	}
	title = titler.Truncate(text, s.pages.InitialTitleWords)
	if r := []rune(title); len(r) > maxTextLen {
		title = string(r[:maxTextLen])
	}
	return title, &text, true
}

// retitle asks the model for a proper title in the background and swaps it
// in for the placeholder, so creating a page never waits on the model.
func (s *Server) retitle(user, id uuid.UUID, placeholder, text string) {
	if !s.pages.Titler.Enabled() {
		return
	}
	go func() {
		// Not the request's context: that is cancelled once the response is sent.
		ctx := context.Background()
		title, err := s.pages.Titler.Generate(ctx, text)
		if err == nil {
			err = s.q.ReplaceTitle(ctx, db.ReplaceTitleParams{UserID: user, ID: id, Placeholder: placeholder, Title: title})
		}
		if err != nil {
			log.Printf("retitle %s: %v", id, err)
		}
	}()
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
	body, ok := decode[struct {
		Text   string `json:"text"`
		IsList bool   `json:"is_list"`
	}](w, r)
	if !ok {
		return
	}
	title, page, ok := s.titleAndPage(w, r, body.Text)
	if !ok {
		return
	}

	var item db.ListsItem
	err := s.inTx(r.Context(), func(q *db.Queries) error {
		var err error
		if body.IsList {
			item, err = createList(r.Context(), q, user, &list.ID, title, page)
		} else {
			item, err = q.CreateItem(r.Context(), db.CreateItemParams{
				UserID: user, ParentID: &list.ID, Text: title, Body: page,
			})
		}
		if err != nil {
			return err
		}
		return q.TouchWithAncestors(r.Context(), list.ID)
	})
	if err != nil {
		serverError(w, err)
		return
	}
	if page != nil {
		s.retitle(user, item.ID, title, *page)
	}
	writeJSON(w, http.StatusCreated, item)
}

// getPage returns a page with the lists above it.
func (s *Server) getPage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	user := auth.UserID(r.Context())
	page, err := s.q.GetItem(r.Context(), db.GetItemParams{ID: id, UserID: user})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && page.Body == nil) {
		writeError(w, http.StatusNotFound, "page not found")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	ancestors, err := s.q.ListAncestors(r.Context(), db.ListAncestorsParams{ID: id, UserID: user})
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"page": page, "ancestors": ancestors})
}

// reorderEntries sets the order of a list's entries to the given id order.
// Ids that are not entries of this list are ignored.
func (s *Server) reorderEntries(w http.ResponseWriter, r *http.Request) {
	user := auth.UserID(r.Context())
	list, ok := s.loadList(w, r, user)
	if !ok {
		return
	}
	body, ok := decode[struct {
		IDs []uuid.UUID `json:"ids"`
	}](w, r)
	if !ok {
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
	body, ok := decode[struct {
		Pinned *bool `json:"pinned"`
	}](w, r)
	if !ok {
		return
	}
	if body.Pinned == nil {
		writeError(w, http.StatusBadRequest, "pinned is required")
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
	body, ok := decode[struct {
		Completed *bool `json:"completed"`
	}](w, r)
	if !ok {
		return
	}
	if body.Completed == nil {
		writeError(w, http.StatusBadRequest, "completed is required")
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

// updateItem changes an item's text, a page's body, or turns an entry into a list.
// A list cannot be turned back into an entry: its contents would be orphaned.
func (s *Server) updateItem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	body, ok := decode[struct {
		Text   *string `json:"text"`
		IsList *bool   `json:"is_list"`
		Body   *string `json:"body"`
	}](w, r)
	if !ok {
		return
	}
	makeList := body.IsList != nil && *body.IsList
	if body.IsList != nil && !makeList {
		writeError(w, http.StatusUnprocessableEntity, "a list cannot be turned back into an entry")
		return
	}
	if body.Text == nil && body.Body == nil && !makeList {
		writeError(w, http.StatusUnprocessableEntity, "nothing to update")
		return
	}
	var text string
	if body.Text != nil {
		if text, ok = validText(w, *body.Text); !ok {
			return
		}
	}

	var pageText string
	if body.Body != nil {
		pageText = strings.TrimSpace(*body.Body)
		if pageText == "" || len([]rune(pageText)) > s.pages.MaxChars {
			writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("page text must be 1 to %d characters", s.pages.MaxChars))
			return
		}
	}

	user := auth.UserID(r.Context())
	var item db.ListsItem
	err := s.inTx(r.Context(), func(q *db.Queries) error {
		var err error
		if body.Body != nil {
			if item, err = q.UpdatePageBody(r.Context(), db.UpdatePageBodyParams{ID: id, UserID: user, Body: pageText}); err != nil {
				return err
			}
		}
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

// decode reads a JSON request body, answering 400 itself if it is malformed.
func decode[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var v T
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return v, false
	}
	return v, true
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
