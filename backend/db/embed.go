// Package migrations embeds the SQL schema so the server can apply it on startup.
package migrations

import (
	"context"
	"embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// 000_schema.sql is admin-only provisioning and is deliberately not matched.
//
//go:embed migrations/00[1-9]_*.sql
var files embed.FS

// Apply runs every migration in filename order. Each one is idempotent.
func Apply(ctx context.Context, pool *pgxpool.Pool) error {
	entries, err := files.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, e := range entries {
		sql, err := files.ReadFile("migrations/" + e.Name())
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("%s: %w", e.Name(), err)
		}
	}
	return nil
}
