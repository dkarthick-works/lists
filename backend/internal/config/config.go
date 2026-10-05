// Package config loads server settings from the environment.
package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL   string
	Port          string
	GoauthBaseURL string
	// JWTSecret is Goauth's signing secret. Optional: without it every token is
	// verified by calling Goauth, which is much slower.
	JWTSecret string
}

func Load() (Config, error) {
	c := Config{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		Port:          os.Getenv("PORT"),
		GoauthBaseURL: os.Getenv("GOAUTH_BASE_URL"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
	}
	if c.Port == "" {
		c.Port = "8082"
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL is required")
	}
	if c.GoauthBaseURL == "" {
		return c, fmt.Errorf("GOAUTH_BASE_URL is required")
	}
	return c, nil
}
