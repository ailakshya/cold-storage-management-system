package templates

import "embed"

// FS embeds all HTML template files into the binary
// This allows the server to run standalone without external template files
//
//go:embed *.html
var FS embed.FS
