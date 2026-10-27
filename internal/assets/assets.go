package assets

import "embed"

//go:embed demo.lua
var DemoSource string

//go:embed demo.lua
var FS embed.FS
