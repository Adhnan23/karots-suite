// Package spa serves the built React app (web/ → internal/spa/dist) from the
// binary. `make web` fills dist; an unbuilt tree still compiles (.gitkeep).
package spa

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

func Dist() fs.FS {
	sub, _ := fs.Sub(dist, "dist")
	return sub
}
