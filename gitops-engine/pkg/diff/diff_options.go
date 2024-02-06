package diff

import (
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/klog/v2/klogr"
)

type Option func(*options)

// Holds diffing settings
type options struct {
	// If set to true then differences caused by aggregated roles in RBAC resources are ignored.
	ignoreAggregatedRoles bool
	normalizer            Normalizer
	log                   logr.Logger
	structuredMergeDiff   bool
	gvkParser             *managedfields.GvkParser
	manager               string
}

func applyOptions(opts []Option) options {
	o := options{
		ignoreAggregatedRoles: false,
		normalizer:            GetNoopNormalizer(),
		log:                   klogr.New(),
	}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

func IgnoreAggregatedRoles(ignore bool) Option {
	return func(o *options) {
		o.ignoreAggregatedRoles = ignore
	}
}

func WithNormalizer(normalizer Normalizer) Option {
	return func(o *options) {
		o.normalizer = normalizer
	}
}

func WithLogr(log logr.Logger) Option {
	return func(o *options) {
		o.log = log
	}
}

func WithStructuredMergeDiff(smd bool) Option {
	return func(o *options) {
		o.structuredMergeDiff = smd
	}
}

func WithGVKParser(parser *managedfields.GvkParser) Option {
	return func(o *options) {
		o.gvkParser = parser
	}
}

func WithManager(manager string) Option {
	return func(o *options) {
		o.manager = manager
	}
}
