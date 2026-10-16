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

// The handlers below are thin HTTP wrappers; the methods they call hold the
// app rules so other callers (such as MCP tools) can share them.

// errStatus is a failure the caller should see, with the HTTP status and
// message it maps to.
type errStatus struct {
	status int
	msg    string
}

func (e *errStatus) Error() string { return e.msg }

var errListNotFound = &errStatus{http.StatusNotFound, "list not found"}

// fail answers err with its own status if it is an errStatus, else 500.
func fail(w http.ResponseWriter, err error) {
	var e *errStatus
	if errors.As(err, &e) {
		writeError(w, e.status, e.msg)
		return
	}
	serverError(w, err)
}

// listLists returns every top-level list.
func (s *Server) listLists(w http.ResponseWriter, r *http.Request) {
	rows, err := s.topLists(r.Context(), auth.UserID(r.Context()))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *Server) topLists(ctx context.Context, user uuid.UUID) ([]db.ListsItem, error) {
	return s.q.ListTopLevel(ctx, user)
}

// autocompleteLists returns the best few list-title matches for ?q=.
func (s *Server) autocompleteLists(w http.ResponseWriter, r *http.Request) {
	rows, err := s.searchLists(r.Context(), auth.UserID(r.Context()), r.URL.Query().Get("q"))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

// searchLists returns the best few list-title matches for q.
func (s *Server) searchLists(ctx context.Context, user uuid.UUID, q string) ([]db.AutocompleteListsRow, error) {
	q = strings.TrimSpace(q)
	if q == "" || len([]rune(q)) > maxTextLen {
		return []db.AutocompleteListsRow{}, nil
	}
	// The query is matched literally, so neutralise LIKE wildcards.
	return s.q.AutocompleteLists(ctx, db.AutocompleteListsParams{
		UserID: user, Query: likeEscaper.Replace(q),
	})
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
	body, ok := decode[struct {
		Title string `json:"title"`
	}](w, r)
	if !ok {
		return
	}
	list, err := s.newList(r.Context(), auth.UserID(r.Context()), body.Title)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, list)
}

// newList makes a top-level list. Long text becomes a short title, with the
// full text kept as a page inside the new list.
func (s *Server) newList(ctx context.Context, user uuid.UUID, text string) (db.ListsItem, error) {
	title, page, err := s.titleAndPage(text)
	if err != nil {
		return db.ListsItem{}, err
	}
	var list db.ListsItem
	err = s.inTx(ctx, func(q *db.Queries) error {
		var err error
		list, err = insertList(ctx, q, user, nil, title, page)
		return err
	})
	if err != nil {
		return db.ListsItem{}, err
	}
	if page != nil {
		s.retitle(user, list.ID, title, *page)
	}
	return list, nil
}

// insertList inserts a list and, when page is set, a page inside it.
func insertList(ctx context.Context, q *db.Queries, user uuid.UUID, parent *uuid.UUID, title string, page *string) (db.ListsItem, error) {
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
func (s *Server) titleAndPage(raw string) (title string, page *string, err error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return "", nil, &errStatus{http.StatusUnprocessableEntity, "text is required"}
	}
	runes := len([]rune(text))
	if len(strings.Fields(text)) <= s.pages.WordThreshold && runes <= maxTextLen {
		return text, nil, nil
	}
	if runes > s.pages.MaxChars {
		return "", nil, &errStatus{http.StatusUnprocessableEntity, fmt.Sprintf("text is too long (limit %d characters)", s.pages.MaxChars)}
	}
	title = titler.Truncate(text, s.pages.InitialTitleWords)
	if r := []rune(title); len(r) > maxTextLen {
		title = string(r[:maxTextLen])
	}
	return title, &text, nil
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
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	view, err := s.openList(r.Context(), auth.UserID(r.Context()), id)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// listView is a list with its entries and the lists above it.
type listView struct {
	Ancestors []db.ListAncestorsRow `json:"ancestors"`
	Entries   []db.ListsItem        `json:"entries"`
	List      db.ListsItem          `json:"list"`
}

func (s *Server) openList(ctx context.Context, user, id uuid.UUID) (listView, error) {
	list, err := s.list(ctx, user, id)
	if err != nil {
		return listView{}, err
	}
	entries, err := s.q.ListEntries(ctx, db.ListEntriesParams{ParentID: &list.ID, UserID: user})
	if err != nil {
		return listView{}, err
	}
	ancestors, err := s.q.ListAncestors(ctx, db.ListAncestorsParams{ID: list.ID, UserID: user})
	if err != nil {
		return listView{}, err
	}
	return listView{List: list, Entries: entries, Ancestors: ancestors}, nil
}

func (s *Server) createEntry(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
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
	item, err := s.addEntry(r.Context(), auth.UserID(r.Context()), id, body.Text, body.IsList)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// addEntry adds an entry, or a sublist when isList, to the end of a list.
// Long text becomes a page, as in newList.
func (s *Server) addEntry(ctx context.Context, user, listID uuid.UUID, text string, isList bool) (db.ListsItem, error) {
	list, err := s.list(ctx, user, listID)
	if err != nil {
		return db.ListsItem{}, err
	}
	title, page, err := s.titleAndPage(text)
	if err != nil {
		return db.ListsItem{}, err
	}

	var item db.ListsItem
	err = s.inTx(ctx, func(q *db.Queries) error {
		var err error
		if isList {
			item, err = insertList(ctx, q, user, &list.ID, title, page)
		} else {
			item, err = q.CreateItem(ctx, db.CreateItemParams{
				UserID: user, ParentID: &list.ID, Text: title, Body: page,
			})
		}
		if err != nil {
			return err
		}
		return q.TouchWithAncestors(ctx, list.ID)
	})
	if err != nil {
		return db.ListsItem{}, err
	}
	if page != nil {
		s.retitle(user, item.ID, title, *page)
	}
	return item, nil
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
	item, err := s.list(r.Context(), user, id)
	if err != nil {
		fail(w, err)
		return db.ListsItem{}, false
	}
	return item, true
}

// list fetches the user's list id, failing with errListNotFound for a
// missing item or a plain entry.
func (s *Server) list(ctx context.Context, user, id uuid.UUID) (db.ListsItem, error) {
	item, err := s.q.GetItem(ctx, db.GetItemParams{ID: id, UserID: user})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !item.IsList) {
		return db.ListsItem{}, errListNotFound
	}
	return item, err
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
