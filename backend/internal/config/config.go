// Package config loads server settings from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL   string
	Port          string
	GoauthBaseURL string
	// JWTSecret is the secret Goauth signs access tokens with.
	JWTSecret string

	// PageWordThreshold: text with more words than this becomes a page.
	PageWordThreshold int
	// InitialTitleWords: a new page is titled with this many of its opening
	// words until the generated title arrives.
	InitialTitleWords int
	// TitleMaxWords is the length of the short title generated for a page.
	TitleMaxWords int
	// PageMaxChars is the longest page text accepted.
	PageMaxChars int

	// OpenRouter generates page titles. Without a key and model, pages keep
	// their initial title.
	OpenRouterAPIKey  string
	OpenRouterModel   string
	OpenRouterBaseURL string
	OpenRouterTimeout time.Duration
}

func Load() (Config, error) {
	c := Config{
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		Port:              env("PORT", "8082"),
		GoauthBaseURL:     os.Getenv("GOAUTH_BASE_URL"),
		JWTSecret:         os.Getenv("JWT_SECRET"),
		OpenRouterAPIKey:  os.Getenv("OPENROUTER_API_KEY"),
		OpenRouterModel:   os.Getenv("OPENROUTER_MODEL"),
		OpenRouterBaseURL: env("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"),
	}
	for name, v := range map[string]string{
		"DATABASE_URL": c.DatabaseURL, "GOAUTH_BASE_URL": c.GoauthBaseURL, "JWT_SECRET": c.JWTSecret,
	} {
		if v == "" {
			return c, fmt.Errorf("%s is required", name)
		}
	}

	var err error
	if c.PageWordThreshold, err = positiveInt("PAGE_WORD_THRESHOLD", "20"); err != nil {
		return c, err
	}
	if c.InitialTitleWords, err = positiveInt("PAGE_INITIAL_TITLE_WORDS", "10"); err != nil {
		return c, err
	}
	if c.TitleMaxWords, err = positiveInt("TITLE_MAX_WORDS", "6"); err != nil {
		return c, err
	}
	if c.PageMaxChars, err = positiveInt("PAGE_MAX_CHARS", "20000"); err != nil {
		return c, err
	}
	if c.OpenRouterTimeout, err = time.ParseDuration(env("OPENROUTER_TIMEOUT", "15s")); err != nil {
		return c, fmt.Errorf("OPENROUTER_TIMEOUT: %w", err)
	}
	return c, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func positiveInt(key, fallback string) (int, error) {
	n, err := strconv.Atoi(env(key, fallback))
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%s must be a positive whole number", key)
	}
	return n, nil
}
