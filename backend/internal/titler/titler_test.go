package titler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTruncate(t *testing.T) {
	if got := Truncate("one two three four", 3); got != "one two three…" {
		t.Errorf("cut: got %q", got)
	}
	if got := Truncate("short  text", 3); got != "short text" {
		t.Errorf("no cut: got %q", got)
	}
}

func TestGenerate(t *testing.T) {
	if New("", "m", "", 3, time.Second).Enabled() {
		t.Error("enabled without a key")
	}

	// The model's answer is cleaned up.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("unexpected request: %s %s", r.URL.Path, r.Header.Get("Authorization"))
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"  \"Counting To Eight.\"\n"}}]}`))
	}))
	defer srv.Close()
	got, err := New("k", "m", srv.URL, 3, time.Second).Generate(context.Background(), "one two three four")
	if err != nil || got != "Counting To Eight" {
		t.Errorf("model: got %q, %v", got, err)
	}

	// Failures and empty answers are errors, so callers keep their placeholder.
	for name, reply := range map[string]http.HandlerFunc{
		"bad gateway": func(w http.ResponseWriter, r *http.Request) { http.Error(w, "nope", http.StatusBadGateway) },
		"empty": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"choices":[{"message":{"content":null}}]}`))
		},
	} {
		bad := httptest.NewServer(reply)
		if _, err := New("k", "m", bad.URL, 3, time.Second).Generate(context.Background(), "x"); err == nil {
			t.Errorf("%s: want an error", name)
		}
		bad.Close()
	}
}
