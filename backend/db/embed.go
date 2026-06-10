// Package db embeds the tern SQL migrations so they ship inside the binary (no files
// needed at runtime). cmd/migrate applies them via the tern migrate package.
package db

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrations returns the migration files as an fs.FS rooted at the migration directory.
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		panic(err) // embed path is a compile-time constant; this cannot fail
	}
	return sub
}
