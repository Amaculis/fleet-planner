// Package web embeds the static assets into the binary. The runtime image is
// distroless and read-only, so there is no asset directory to mount or to tamper with.
package web

import (
	"embed"
	"io/fs"
)

//go:embed static
var staticFiles embed.FS

// StaticFS returns the contents of web/static rooted at "/".
func StaticFS() fs.FS {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		// Only possible if the embed directive above is broken at build time.
		panic("web: static assets not embedded: " + err.Error())
	}
	return sub
}
