// Package migrations embeds the SQL schema so the server can apply it on startup.
package migrations

import _ "embed"

//go:embed migrations/001_init.sql
var initSchema string

//go:embed migrations/002_trigram_search.sql
var trigramSearch string

// All is every migration the server applies, in order. Each is idempotent.
var All = []string{initSchema, trigramSearch}
