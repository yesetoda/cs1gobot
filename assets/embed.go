package assets

import "embed"

// Files embeds all robot sprite PNGs so the UI can load images reliably
// regardless of the current working directory.
//
//go:embed *.png
var Files embed.FS
