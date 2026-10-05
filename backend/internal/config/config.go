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
}

func Load() (Config, error) {
	c := Config{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		Port:          os.Getenv("PORT"),
		GoauthBaseURL: os.Getenv("GOAUTH_BASE_URL"),
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
