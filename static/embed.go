package static

import "embed"

// FS embeds all static files (CSS, JS, fonts, images) into the binary
// This allows the server to run standalone without external static files
//
//go:embed css fonts js
var FS embed.FS
