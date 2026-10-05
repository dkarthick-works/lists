package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const testSecret = "test-secret-not-used-anywhere-real"

func sign(t *testing.T, method jwt.SigningMethod, key any, claims jwt.RegisteredClaims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestVerifyLocal(t *testing.T) {
	v := NewVerifier("http://goauth.invalid", testSecret)
	user := uuid.New()
	in := func(d time.Duration) *jwt.NumericDate { return jwt.NewNumericDate(time.Now().Add(d)) }
	valid := jwt.RegisteredClaims{Subject: user.String(), ExpiresAt: in(15 * time.Minute)}

	got, err := v.verifyLocal(sign(t, jwt.SigningMethodHS256, []byte(testSecret), valid))
	if err != nil || got != user {
		t.Fatalf("valid token: got %v, %v", got, err)
	}

	rejected := map[string]string{
		"expired":       sign(t, jwt.SigningMethodHS256, []byte(testSecret), jwt.RegisteredClaims{Subject: user.String(), ExpiresAt: in(-time.Minute)}),
		"no expiry":     sign(t, jwt.SigningMethodHS256, []byte(testSecret), jwt.RegisteredClaims{Subject: user.String()}),
		"wrong secret":  sign(t, jwt.SigningMethodHS256, []byte("another-secret"), valid),
		"unsigned":      sign(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, valid),
		"other hmac":    sign(t, jwt.SigningMethodHS512, []byte(testSecret), valid),
		"non-uuid user": sign(t, jwt.SigningMethodHS256, []byte(testSecret), jwt.RegisteredClaims{Subject: "admin", ExpiresAt: in(time.Minute)}),
		"garbage":       "not.a.token",
	}
	for name, token := range rejected {
		if _, err := v.verifyLocal(token); err != ErrUnauthorized {
			t.Errorf("%s: want ErrUnauthorized, got %v", name, err)
		}
	}
}
