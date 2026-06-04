package resource_customizations

import (
	"embed"
)

// Embedded contains embedded resource customization
//
//go:embed all:*
var Embedded embed.FS
