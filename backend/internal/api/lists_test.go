package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTitleAndPage(t *testing.T) {
	s := &Server{pages: Pages{WordThreshold: 3, InitialTitleWords: 2, MaxChars: 40}}

	if title, page, err := s.titleAndPage("  milk eggs  "); err != nil || title != "milk eggs" || page != nil {
		t.Errorf("short: %q %v %v", title, page, err)
	}
	title, page, err := s.titleAndPage("one two three four")
	if err != nil || page == nil || *page != "one two three four" || title == *page {
		t.Errorf("long: %q %v %v", title, page, err)
	}

	for raw, want := range map[string]string{
		"   ":                   "text is required",
		strings.Repeat("a ", 30): "text is too long (limit 40 characters)",
	} {
		_, _, err := s.titleAndPage(raw)
		var e *errStatus
		if !errors.As(err, &e) || e.status != http.StatusUnprocessableEntity || e.msg != want {
			t.Errorf("%q: got %v", raw, err)
		}
	}
}

func TestFail(t *testing.T) {
	w := httptest.NewRecorder()
	fail(w, errListNotFound)
	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "list not found") {
		t.Errorf("errStatus: %d %s", w.Code, w.Body)
	}
	w = httptest.NewRecorder()
	fail(w, errors.New("db down"))
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "internal server error") {
		t.Errorf("other: %d %s", w.Code, w.Body)
	}
}
