package static

import "embed"

// FS embeds all static files (CSS, JS, fonts, icons, PWA) into the binary
// This allows the server to run standalone without external static files
//
// Cache bust: v2
//go:embed css fonts js icons locales manifest.json sw.js favicon.ico
var FS embed.FS
