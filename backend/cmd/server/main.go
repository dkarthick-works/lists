package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	migrations "lists/db"
	"lists/internal/api"
	"lists/internal/auth"
	"lists/internal/config"
	"lists/internal/titler"
)

func main() {
	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	// Loosen the cutoff for the fuzzy half of list autocomplete (the <% operator);
	// the 0.6 default rejects most single-typo queries.
	poolCfg.ConnConfig.RuntimeParams["pg_trgm.word_similarity_threshold"] = "0.4"
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := migrations.Apply(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	authProxy, err := auth.NewProxy(cfg.GoauthBaseURL)
	if err != nil {
		log.Fatal(err)
	}
	srv := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: api.NewRouter(pool, auth.NewVerifier(cfg.JWTSecret), authProxy, api.Pages{
			WordThreshold:     cfg.PageWordThreshold,
			InitialTitleWords: cfg.InitialTitleWords,
			MaxChars:          cfg.PageMaxChars,
			Titler: titler.New(cfg.OpenRouterAPIKey, cfg.OpenRouterModel, cfg.OpenRouterBaseURL,
				cfg.TitleMaxWords, cfg.OpenRouterTimeout),
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("listening on :%s", cfg.Port)
	log.Fatal(srv.ListenAndServe())
}
