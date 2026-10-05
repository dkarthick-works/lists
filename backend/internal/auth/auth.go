// Package auth identifies the caller from a Goauth access token and proxies
// the browser's auth calls through to Goauth.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type ctxKey int

const userKey ctxKey = iota

var ErrUnauthorized = errors.New("unauthorized")

// cacheTTL bounds how long a token stays accepted after Goauth last vouched
// for it (fallback mode only). Access tokens live 15 minutes, so this adds at most a minute.
const cacheTTL = time.Minute

type cached struct {
	userID  uuid.UUID
	expires time.Time
}

// Verifier resolves access tokens to user ids via Goauth's GET /auth/me.
// Verifier resolves access tokens to user ids. With the Goauth signing secret
// it checks the token itself; without one it falls back to asking Goauth's
// GET /auth/me, which costs a network round trip per token per minute.
type Verifier struct {
	secret []byte

	meURL  string
	client *http.Client

	mu    sync.Mutex
	cache map[string]cached
}

// NewVerifier takes Goauth's JWT_SECRET; pass "" to verify through Goauth instead.
func NewVerifier(goauthBaseURL, jwtSecret string) *Verifier {
	return &Verifier{
		secret: []byte(jwtSecret),
		meURL:  strings.TrimRight(goauthBaseURL, "/") + "/auth/me",
		client: &http.Client{Timeout: 10 * time.Second},
		cache:  map[string]cached{},
	}
}

// Local reports whether tokens are verified in-process.
func (v *Verifier) Local() bool { return len(v.secret) > 0 }

// verifyLocal checks an HS256 token's signature and expiry and returns its
// subject, which Goauth sets to the user id.
func (v *Verifier) verifyLocal(token string) (uuid.UUID, error) {
	claims := jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(token, &claims, func(*jwt.Token) (any, error) {
		return v.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil {
		return uuid.Nil, ErrUnauthorized
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, ErrUnauthorized
	}
	return id, nil
}

func (v *Verifier) lookup(ctx context.Context, token string) (uuid.UUID, error) {
	if v.Local() {
		return v.verifyLocal(token)
	}

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
