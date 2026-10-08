package titler

import (
	"context"
	"os"
	"testing"
	"time"
)

// Calls the real OpenRouter API. Skipped unless OPENROUTER_API_KEY and
// OPENROUTER_MODEL are set: `set -a; . ./.env; set +a; go test ./internal/titler -run Live -v`
func TestLive(t *testing.T) {
	key, model := os.Getenv("OPENROUTER_API_KEY"), os.Getenv("OPENROUTER_MODEL")
	if key == "" || model == "" {
		t.Skip("OPENROUTER_API_KEY / OPENROUTER_MODEL not set")
	}
	tl := New(key, model, "https://openrouter.ai/api/v1", 6, 15*time.Second)
	note := "The internet looks like magic. Underneath, it is mostly computers passing small packets of data to each other along cables that run under the sea."
	title, err := tl.Generate(context.Background(), note)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("title: %q", title)
}
