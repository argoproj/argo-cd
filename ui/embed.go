package ui

import "embed"

// Embedded contains embedded UI resources
//
//go:embed dist/app
var Embedded embed.FS
