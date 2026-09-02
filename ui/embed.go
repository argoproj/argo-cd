package ui

import "embed"

// Embedded contains embedded UI resources
//
//go:embed dist/app
//go:embed all:dist/app/assets/images/resources
var Embedded embed.FS
