// Package web contains the server-rendered Mystic Square web UI.
package web

import (
	"embed"
	"io/fs"
)

// The web UI is embedded so the API and its pages can be distributed as one
// binary. Keep these directories rooted at their respective subdirectories;
// callers should not need to know the layout of the embedded filesystem.
//
//go:embed templates static
var embeddedFS embed.FS

// TemplatesFS contains the embedded templates rooted at web/templates.
var TemplatesFS = subFS("templates")

// StaticFS contains the embedded assets rooted at web/static.
var StaticFS = subFS("static")

func subFS(name string) fs.FS {
	files, err := fs.Sub(embeddedFS, name)
	if err != nil {
		panic("web: invalid embedded filesystem: " + err.Error())
	}
	return files
}
