package web

import (
    "embed"
    "io/fs"
)

//go:embed dist
var distAssets embed.FS

func FS() (fs.FS, error) {
    return fs.Sub(distAssets, "dist")
}
