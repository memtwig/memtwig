package ui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var rawFS embed.FS

// FS returns the embedded static build of Memtwig Studio (web/).
func FS() fs.FS {
	sub, err := fs.Sub(rawFS, "dist")
	if err != nil {
		return nil
	}
	return sub
}
