package webui

import (
	"embed"
	"io/fs"
)

//go:generate sh ../tools/generate_web.sh

// embeddedAssets contains the statically generated Nuxt SPA output.
//
// The all: prefix is required so go:embed includes dot-prefixed directories
// like .output and underscore-prefixed paths like /_nuxt.
//
//go:embed all:.output/public
var embeddedAssets embed.FS

// AssetsFS returns the generated SPA output rooted at .output/public.
func AssetsFS() (fs.FS, error) {
	return fs.Sub(embeddedAssets, ".output/public")
}
