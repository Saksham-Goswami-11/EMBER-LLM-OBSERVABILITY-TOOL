// Package web embeds the built dashboard so the Go binary needs nothing
// else on disk to serve it. dist/ is checked in with a placeholder
// (dist/.gitkeep) so a fresh clone always compiles; `npm run build`
// overwrites it with the real assets.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distEmbed embed.FS

// FS returns the dashboard's static files rooted at dist/.
func FS() fs.FS {
	sub, err := fs.Sub(distEmbed, "dist")
	if err != nil {
		panic(err) // dist/ is embedded above; this cannot fail
	}
	return sub
}
