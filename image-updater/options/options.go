package options

import (
	"sort"
	"sync"

	"github.com/argoproj/argo-cd/v2/image-updater/log"
)

// ManifestOptions define some options when retrieving image manifests
type ManifestOptions struct {
	platforms map[string]bool
	mutex     sync.RWMutex
	metadata  bool
	logger    *log.LogContext
}

// NewManifestOptions returns an initialized ManifestOptions struct
func NewManifestOptions() *ManifestOptions {
	return &ManifestOptions{
		platforms: make(map[string]bool),
		metadata:  false,
	}
}

// PlatformKey returns a string usable as platform key
func PlatformKey(os string, arch string, variant string) string {
	key := os + "/" + arch
	if variant != "" {
		key += "/" + variant
	}
	return key
}

// MatchesPlatform returns true if given OS name matches the OS set in options
func (o *ManifestOptions) WantsPlatform(os string, arch string, variant string) bool {
	o.mutex.RLock()
	defer o.mutex.RUnlock()
	if len(o.platforms) == 0 {
		return true
	}
	_, ok := o.platforms[PlatformKey(os, arch, variant)]
	return ok
}

// WithPlatform sets a platform filter for options o
func (o *ManifestOptions) WithPlatform(os string, arch string, variant string) *ManifestOptions {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	if o.platforms == nil {
		o.platforms = map[string]bool{}
	}
	o.platforms[PlatformKey(os, arch, variant)] = true
	return o
}

func (o *ManifestOptions) Platforms() []string {
	o.mutex.RLock()
	defer o.mutex.RUnlock()
	if len(o.platforms) == 0 {
		return []string{}
	}
	keys := make([]string, 0, len(o.platforms))
	for k := range o.platforms {
		keys = append(keys, k)
	}
	// We sort the slice before returning it, to guarantee stable order
	sort.Strings(keys)
	return keys
}

// WantsMetdata returns true if metadata should be requested
func (o *ManifestOptions) WantsMetadata() bool {
	return o.metadata
}

// WithMetadata sets metadata to be requested
func (o *ManifestOptions) WithMetadata(val bool) *ManifestOptions {
	o.metadata = val
	return o
}

// WithLogger sets the log context to use for the given manifest options.
func (o *ManifestOptions) WithLogger(logger *log.LogContext) *ManifestOptions {
	o.logger = logger
	return o
}

// Logger gets the configured log context for given manifest options. If logger
// is nil, returns a default log context.
func (o *ManifestOptions) Logger() *log.LogContext {
	if o.logger == nil {
		return log.WithContext()
	} else {
		return o.logger
	}
}
