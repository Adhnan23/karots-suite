// Package ui is the Demo Room's built pages and chrome (web/demo → dist).
package ui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// FS is the built files, or nil when only the placeholder is there (a plain
// `go build` before `make demo-web`).
func FS() fs.FS {
	sub, _ := fs.Sub(dist, "dist")
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil
	}
	return sub
}
