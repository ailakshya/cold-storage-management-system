package migrations

import "embed"

// FS embeds all SQL migration files into the binary
// This allows the server to run standalone without external migration files
//
//go:embed *.sql
var FS embed.FS
