package cache

import (
	"github.com/argoproj/argo-cd/v2/image-updater/tag"
)

type ImageTagCache interface {
	HasTag(imageName string, imageTag string) bool
	GetTag(imageName string, imageTag string) (*tag.ImageTag, error)
	SetTag(imageName string, imgTag *tag.ImageTag)
	ClearCache()
	NumEntries() int
}

// KnownImage represents a known image and the applications using it, without
// any version/tag information.
type KnownImage struct {
	ImageName    string
	Applications []string
}
