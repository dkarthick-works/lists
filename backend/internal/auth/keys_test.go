package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestKeyMiddleware(t *testing.T) {
	user := uuid.New()
	key, hash := NewKey()
	lookup := func(_ context.Context, h string) (uuid.UUID, error) {
		if h == hash {
			return user, nil
		}
		return uuid.Nil, errors.New("no rows")
	}
	var got uuid.UUID
	h := KeyMiddleware(lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = UserID(r.Context())
	}))

	for header, want := range map[string]int{
		"":                      http.StatusUnauthorized,
		"Bearer ":               http.StatusUnauthorized,
		"Bearer lst_unknown":    http.StatusUnauthorized,
		"Bearer " + key[4:]:     http.StatusUnauthorized, // missing prefix
		"Basic " + key:          http.StatusUnauthorized,
		"Bearer " + key:         http.StatusOK,
		"bearer  " + key + "  ": http.StatusOK,
	} {
		got = uuid.Nil
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", header)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("%q: status %d, want %d", header, rec.Code, want)
		}
		if want == http.StatusOK && got != user {
			t.Errorf("%q: user %v, want %v", header, got, user)
		}
	}
}
