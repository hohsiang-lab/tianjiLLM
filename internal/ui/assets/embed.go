package assets

import "embed"

//go:embed all:css all:js
var Static embed.FS
