package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// keyPrefix marks a personal API key, telling it apart from a Goauth token.
const keyPrefix = "lst_"

// NewKey returns a fresh API key and the hash to store for it.
func NewKey() (key, hash string) {
	b := make([]byte, 32)
	rand.Read(b) // never fails since Go 1.24
	key = keyPrefix + base64.RawURLEncoding.EncodeToString(b)
	return key, HashKey(key)
}

// HashKey is the stored form of an API key: the SHA-256 hex of the full key.
func HashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// KeyMiddleware requires a Bearer API key and stores its owner in context,
// so UserID works as it does behind Middleware. lookup resolves a key hash to
// the user of a live key and errors for unknown or revoked keys.
func KeyMiddleware(lookup func(ctx context.Context, keyHash string) (uuid.UUID, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if len(h) <= 7 || !strings.EqualFold(h[:7], "Bearer ") {
				WriteError(w, http.StatusUnauthorized, "missing API key")
				return
			}
			key := strings.TrimSpace(h[7:])
			if !strings.HasPrefix(key, keyPrefix) {
				WriteError(w, http.StatusUnauthorized, "invalid API key")
				return
			}
			id, err := lookup(r.Context(), HashKey(key))
			if err != nil {
				WriteError(w, http.StatusUnauthorized, "invalid API key")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, id)))
		})
	}
}
