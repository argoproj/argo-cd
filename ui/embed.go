package ui

import "embed"

// Embedded contains embedded UI resources
//
//go:embed all:dist/app
var Embedded embed.FS
