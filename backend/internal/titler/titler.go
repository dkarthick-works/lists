// Package titler writes short titles for long text: a plain truncation, and
// a better one from an LLM through OpenRouter when one is configured.
package titler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// maxPromptRunes caps how much of the text is sent to the model; a title
// does not need more than the opening of a long note.
const maxPromptRunes = 4000

type Titler struct {
	// MaxWords is the title length asked of the model.
	MaxWords int
	// APIKey and Model enable the model; with either empty, Enabled is false.
	APIKey, Model, BaseURL string
	Client                 *http.Client
}

func New(apiKey, model, baseURL string, maxWords int, timeout time.Duration) *Titler {
	return &Titler{
		MaxWords: maxWords, APIKey: apiKey, Model: model,
		BaseURL: strings.TrimRight(baseURL, "/"),
		Client:  &http.Client{Timeout: timeout},
	}
}

// Enabled reports whether a model is configured.
func (t *Titler) Enabled() bool { return t.APIKey != "" && t.Model != "" }

// Truncate returns the first n words of text, with an ellipsis if any were cut.
func Truncate(text string, n int) string {
	words := strings.Fields(text)
	if len(words) <= n {
		return strings.Join(words, " ")
	}
	return strings.Join(words[:n], " ") + "…"
}

// Generate asks the model for a title of at most MaxWords words.
func (t *Titler) Generate(ctx context.Context, text string) (string, error) {
	if r := []rune(text); len(r) > maxPromptRunes {
		text = string(r[:maxPromptRunes])
	}
	reqBody, _ := json.Marshal(map[string]any{
		"model": t.Model,
		"messages": []map[string]string{
			{"role": "system", "content": fmt.Sprintf(
				"You write titles for notes. Reply with a plain title of at most %d words that captures what the note is about. No quotes, no trailing punctuation, nothing else.", t.MaxWords)},
			{"role": "user", "content": text},
		},
		// Reasoning models otherwise spend the whole budget thinking and return
		// no title at all. A title needs no reasoning, so switch it off.
		"reasoning":  map[string]bool{"enabled": false},
		"max_tokens": 60,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.BaseURL+"/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+t.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openrouter: %s", resp.Status)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("openrouter: no choices")
	}
	// Models sometimes add quotes or run long despite the instruction.
	title := strings.Trim(strings.Join(strings.Fields(out.Choices[0].Message.Content), " "), `"'“”.`)
	if title == "" {
		return "", fmt.Errorf("openrouter: empty title from %s", t.Model)
	}
	return Truncate(title, t.MaxWords*2), nil
}
