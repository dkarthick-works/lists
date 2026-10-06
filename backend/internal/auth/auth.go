// Package auth identifies the caller from a Goauth access token and proxies
// the browser's auth calls through to Goauth.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type ctxKey int

const userKey ctxKey = iota

var ErrUnauthorized = errors.New("unauthorized")

// Verifier resolves Goauth access tokens to user ids by checking them against
// Goauth's signing secret, with no call to Goauth.
type Verifier struct {
	secret []byte
}

// NewVerifier takes the JWT_SECRET that Goauth signs with.
func NewVerifier(jwtSecret string) *Verifier {
	return &Verifier{secret: []byte(jwtSecret)}
}

// verify checks an HS256 token's signature and expiry and returns its
// subject, which Goauth sets to the user id.
func (v *Verifier) verify(token string) (uuid.UUID, error) {
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

// Middleware requires a valid Bearer token and stores the user id in context.
func (v *Verifier) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if len(h) <= 7 || !strings.EqualFold(h[:7], "Bearer ") {
			WriteError(w, http.StatusUnauthorized, "missing access token")
			return
		}
		id, err := v.verify(strings.TrimSpace(h[7:]))
		if err != nil {
			WriteError(w, http.StatusUnauthorized, "invalid or expired token")
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

// WriteError sends the API's JSON error shape: {"error": msg}.
func WriteError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
