// Package migrations embeds the golang-migrate SQL migration files so the
// service binary can apply them at startup without needing the .sql files
// copied alongside it at runtime.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
