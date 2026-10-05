//go:build !prod

// Package web holds the built frontend, embedded only in prod builds.
package web

import "io/fs"

// FS is nil in non-prod builds; the Vite dev server handles static assets.
var FS fs.FS
