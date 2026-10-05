// Package auth identifies the caller by asking Goauth who an access token
// belongs to, and proxies the browser's auth calls through to Goauth.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type ctxKey int

const userKey ctxKey = iota

var ErrUnauthorized = errors.New("unauthorized")

// cacheTTL bounds how long a token stays accepted after Goauth last vouched
// for it. Access tokens live 15 minutes, so this adds at most a minute.
const cacheTTL = time.Minute

type cached struct {
	userID  uuid.UUID
	expires time.Time
}

// Verifier resolves access tokens to user ids via Goauth's GET /auth/me.
type Verifier struct {
	meURL  string
	client *http.Client

	mu    sync.Mutex
	cache map[string]cached
}

func NewVerifier(goauthBaseURL string) *Verifier {
	return &Verifier{
		meURL:  strings.TrimRight(goauthBaseURL, "/") + "/auth/me",
		client: &http.Client{Timeout: 10 * time.Second},
		cache:  map[string]cached{},
	}
}

func (v *Verifier) lookup(ctx context.Context, token string) (uuid.UUID, error) {
	now := time.Now()
	v.mu.Lock()
	c, ok := v.cache[token]
	v.mu.Unlock()
	if ok && now.Before(c.expires) {
		return c.userID, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.meURL, nil)
	if err != nil {
		return uuid.Nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := v.client.Do(req)
	if err != nil {
		return uuid.Nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return uuid.Nil, ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return uuid.Nil, errors.New("goauth: unexpected status " + resp.Status)
	}
	var me struct {
		UserID uuid.UUID `json:"user_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&me); err != nil || me.UserID == uuid.Nil {
		return uuid.Nil, errors.New("goauth: malformed /auth/me response")
	}

	v.mu.Lock()
	for k, e := range v.cache {
		if now.After(e.expires) {
			delete(v.cache, k)
		}
	}
	v.cache[token] = cached{userID: me.UserID, expires: now.Add(cacheTTL)}
	v.mu.Unlock()
	return me.UserID, nil
}

// Middleware requires a valid Bearer token and stores the user id in context.
func (v *Verifier) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if len(h) <= 7 || !strings.EqualFold(h[:7], "Bearer ") {
			writeError(w, http.StatusUnauthorized, "missing access token")
			return
		}
		id, err := v.lookup(r.Context(), strings.TrimSpace(h[7:]))
		if errors.Is(err, ErrUnauthorized) {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadGateway, "auth service unavailable")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, id)))
	})
}

// UserID returns the authenticated user stored by Middleware.
func UserID(ctx context.Context) uuid.UUID {
	id, _ := ctx.Value(userKey).(uuid.UUID)
	return id
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
