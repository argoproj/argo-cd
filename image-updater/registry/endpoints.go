package registry

import (
	"crypto/tls"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/argoproj/argo-cd/v2/image-updater/cache"
	"github.com/argoproj/argo-cd/v2/image-updater/log"
	"go.uber.org/ratelimit"
)

// TagListSort defines how the registry returns the list of tags
type TagListSort int

const (
	TagListSortUnknown           TagListSort = -1
	TagListSortUnsorted          TagListSort = 0
	TagListSortLatestFirst       TagListSort = 1
	TagListSortLatestLast        TagListSort = 2
	TagListSortUnsortedString    string      = "unsorted"
	TagListSortLatestFirstString string      = "latest-first"
	TagListSortLatestLastString  string      = "latest-last"
	TagListSortUnknownString     string      = "unknown"
)

const (
	RateLimitNone    = math.MaxInt32
	RateLimitDefault = 10
)

// IsTimeSorted returns whether a tag list is time sorted
func (tls TagListSort) IsTimeSorted() bool {
	return tls == TagListSortLatestFirst || tls == TagListSortLatestLast
}

// TagListSortFromString gets the TagListSort value from a given string
func TagListSortFromString(tls string) TagListSort {
	switch strings.ToLower(tls) {
	case "latest-first":
		return TagListSortLatestFirst
	case "latest-last":
		return TagListSortLatestLast
	case "none", "":
		return TagListSortUnsorted
	default:
		log.Warnf("unknown tag list sort mode: %s", tls)
		return TagListSortUnknown
	}
}

// String returns the string representation of a TagListSort value
func (tls TagListSort) String() string {
	switch tls {
	case TagListSortLatestFirst:
		return TagListSortLatestFirstString
	case TagListSortLatestLast:
		return TagListSortLatestLastString
	case TagListSortUnsorted:
		return TagListSortUnsortedString
	}

	return TagListSortUnknownString
}

// RegistryEndpoint holds information on how to access any specific registry API
// endpoint.
type RegistryEndpoint struct {
	RegistryName   string
	RegistryPrefix string
	RegistryAPI    string
	Username       string
	Password       string
	Ping           bool
	Credentials    string
	Insecure       bool
	DefaultNS      string
	CredsExpire    time.Duration
	CredsUpdated   time.Time
	TagListSort    TagListSort
	Cache          cache.ImageTagCache
	Limiter        ratelimit.Limiter
	IsDefault      bool
	lock           sync.RWMutex
	limit          int
}

// registryTweaks should contain a list of registries whose settings cannot be
// infered by just looking at the image prefix. Prominent example here is the
// Docker Hub registry, which is refered to as docker.io from the image, but
// its API endpoint is https://registry-1.docker.io (and not https://docker.io)
var registryTweaks map[string]*RegistryEndpoint = map[string]*RegistryEndpoint{
	"docker.io": {
		RegistryName:   "Docker Hub",
		RegistryPrefix: "docker.io",
		RegistryAPI:    "https://registry-1.docker.io",
		Ping:           true,
		Insecure:       false,
		DefaultNS:      "library",
		Cache:          cache.NewMemCache(),
		Limiter:        ratelimit.New(RateLimitDefault),
		IsDefault:      true,
	},
}

var registries map[string]*RegistryEndpoint = make(map[string]*RegistryEndpoint)

// Default registry points to the registry that is to be used as the default,
// e.g. when no registry prefix is given for a certain image.
var defaultRegistry *RegistryEndpoint

// Simple RW mutex for concurrent access to registries map
var registryLock sync.RWMutex

func AddRegistryEndpointFromConfig(epc RegistryConfiguration) error {
	ep := NewRegistryEndpoint(epc.Prefix, epc.Name, epc.ApiURL, epc.Credentials, epc.DefaultNS, epc.Insecure, TagListSortFromString(epc.TagSortMode), epc.Limit, epc.CredsExpire)
	return AddRegistryEndpoint(ep)
}

// NewRegistryEndpoint returns an endpoint object with the given configuration
// pre-populated and a fresh cache.
func NewRegistryEndpoint(prefix, name, apiUrl, credentials, defaultNS string, insecure bool, tagListSort TagListSort, limit int, credsExpire time.Duration) *RegistryEndpoint {
	if limit <= 0 {
		limit = RateLimitNone
	}
	ep := &RegistryEndpoint{
		RegistryName:   name,
		RegistryPrefix: prefix,
		RegistryAPI:    strings.TrimSuffix(apiUrl, "/"),
		Credentials:    credentials,
		CredsExpire:    credsExpire,
		Cache:          cache.NewMemCache(),
		Insecure:       insecure,
		DefaultNS:      defaultNS,
		TagListSort:    tagListSort,
		Limiter:        ratelimit.New(limit),
		limit:          limit,
	}
	return ep
}

// AddRegistryEndpoint adds registry endpoint information with the given details
func AddRegistryEndpoint(ep *RegistryEndpoint) error {
	prefix := ep.RegistryPrefix

	registryLock.Lock()
	// If the endpoint is supposed to be the default endpoint, make sure that
	// any previously set default endpoint is unset.
	if ep.IsDefault {
		if dep := GetDefaultRegistry(); dep != nil {
			dep.IsDefault = false
		}
		SetDefaultRegistry(ep)
	}
	registries[prefix] = ep
	registryLock.Unlock()

	logCtx := log.WithContext()
	logCtx.AddField("registry", ep.RegistryAPI)
	logCtx.AddField("prefix", ep.RegistryPrefix)
	if ep.limit != RateLimitNone {
		logCtx.Debugf("setting rate limit to %d requests per second", ep.limit)
	} else {
		logCtx.Debugf("rate limiting is disabled")
	}
	return nil
}

// inferRegistryEndpointFromPrefix returns a registry endpoint with the API
// URL infered from the prefix and adds it to the list of the configured
// registries.
func inferRegistryEndpointFromPrefix(prefix string) *RegistryEndpoint {
	apiURL := "https://" + prefix
	return NewRegistryEndpoint(prefix, prefix, apiURL, "", "", false, TagListSortUnsorted, 20, 0)
}

// GetRegistryEndpoint retrieves the endpoint information for the given prefix
func GetRegistryEndpoint(prefix string) (*RegistryEndpoint, error) {
	if prefix == "" {
		if defaultRegistry == nil {
			return nil, fmt.Errorf("no default endpoint configured")
		} else {
			return defaultRegistry, nil
		}
	}

	registryLock.RLock()
	registry, ok := registries[prefix]
	registryLock.RUnlock()

	if ok {
		return registry, nil
	} else {
		var err error
		ep := inferRegistryEndpointFromPrefix(prefix)
		if ep != nil {
			err = AddRegistryEndpoint(ep)
		} else {
			err = fmt.Errorf("could not infer registry configuration from prefix %s", prefix)
		}
		if err == nil {
			log.Debugf("Inferred registry from prefix %s to use API %s", prefix, ep.RegistryAPI)
		}
		return ep, err
	}
}

// SetDefaultRegistry sets a given registry endpoint as the default
func SetDefaultRegistry(ep *RegistryEndpoint) {
	log.Debugf("Setting default registry endpoint to %s", ep.RegistryPrefix)
	ep.IsDefault = true
	if defaultRegistry != nil {
		log.Debugf("Previous default registry was %s", defaultRegistry.RegistryPrefix)
		defaultRegistry.IsDefault = false
	}
	defaultRegistry = ep
}

// GetDefaultRegistry returns the registry endpoint that is set as default,
// or nil if no default registry endpoint is set
func GetDefaultRegistry() *RegistryEndpoint {
	if defaultRegistry != nil {
		log.Debugf("Getting default registry endpoint: %s", defaultRegistry.RegistryPrefix)
	} else {
		log.Debugf("No default registry defined.")
	}
	return defaultRegistry
}

// SetRegistryEndpointCredentials allows to change the credentials used for
// endpoint access for existing RegistryEndpoint configuration
func SetRegistryEndpointCredentials(prefix, credentials string) error {
	registry, err := GetRegistryEndpoint(prefix)
	if err != nil {
		return err
	}
	registry.lock.Lock()
	registry.Credentials = credentials
	registry.lock.Unlock()
	return nil
}

// ConfiguredEndpoints returns a list of prefixes that are configured
func ConfiguredEndpoints() []string {
	r := []string{}
	registryLock.RLock()
	defer registryLock.RUnlock()
	for _, v := range registries {
		r = append(r, v.RegistryPrefix)
	}
	return r
}

// DeepCopy copies the endpoint to a new object, but creating a new Cache
func (ep *RegistryEndpoint) DeepCopy() *RegistryEndpoint {
	ep.lock.RLock()
	newEp := &RegistryEndpoint{}
	newEp.RegistryAPI = ep.RegistryAPI
	newEp.RegistryName = ep.RegistryName
	newEp.RegistryPrefix = ep.RegistryPrefix
	newEp.Credentials = ep.Credentials
	newEp.Ping = ep.Ping
	newEp.TagListSort = ep.TagListSort
	newEp.Cache = cache.NewMemCache()
	newEp.Insecure = ep.Insecure
	newEp.DefaultNS = ep.DefaultNS
	newEp.Limiter = ep.Limiter
	newEp.CredsExpire = ep.CredsExpire
	newEp.CredsUpdated = ep.CredsUpdated
	newEp.IsDefault = ep.IsDefault
	newEp.limit = ep.limit
	ep.lock.RUnlock()
	return newEp
}

// GetTransport returns a transport object for this endpoint
func (ep *RegistryEndpoint) GetTransport() *http.Transport {
	tlsC := &tls.Config{}
	if ep.Insecure {
		tlsC.InsecureSkipVerify = true
	}
	return &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: tlsC,
	}
}

// init initializes the registry configuration
func init() {
	for k, v := range registryTweaks {
		registries[k] = v.DeepCopy()
		if v.IsDefault {
			if defaultRegistry == nil {
				defaultRegistry = v
			} else {
				panic("only one default registry can be configured")
			}
		}
	}
}
