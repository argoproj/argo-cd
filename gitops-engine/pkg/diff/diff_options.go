package diff

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/klog/v2/textlogger"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type Option func(*options)

// Holds diffing settings
type options struct {
	// If set to true then differences caused by aggregated roles in RBAC resources are ignored.
	ignoreAggregatedRoles bool
	normalizer            Normalizer
	skipFullNormalize     bool
	log                   logr.Logger
	gvkParser             *managedfields.GvkParser
	manager               string
	serverSideDiff        bool
	serverSideDryRunner   ServerSideDryRunner
	ignoreMutationWebhook bool
	// trustedManagers lists additional field managers (besides the applier
	// identified by manager) whose fields should be treated the same way
	// mutation-webhook fields are: yielded to rather than flagged as drift.
	// Any manager not in this list, and not the applier itself, is treated
	// as untrusted, so fields it owns are compared normally instead of being
	// unconditionally reverted to their live value. See WithTrustedManagers.
	trustedManagers []string
}

func applyOptions(opts []Option) options {
	o := options{
		ignoreAggregatedRoles: false,
		ignoreMutationWebhook: true,
		normalizer:            GetNoopNormalizer(),
		skipFullNormalize:     false,
		log:                   textlogger.NewLogger(textlogger.NewConfig()),
	}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

type KubeApplier interface {
	ApplyResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, force, validate, serverSideApply bool, manager string) (string, error)
}

// ServerSideDryRunner defines the contract to run a server-side apply in
// dryrun mode.
type ServerSideDryRunner interface {
	Run(ctx context.Context, obj *unstructured.Unstructured, manager string) (string, error)
}

// K8sServerSideDryRunner is the Kubernetes implementation of ServerSideDryRunner.
type K8sServerSideDryRunner struct {
	dryrunApplier KubeApplier
}

// NewK8sServerSideDryRunner will instantiate a new K8sServerSideDryRunner with
// the given kubeApplier.
func NewK8sServerSideDryRunner(kubeApplier KubeApplier) *K8sServerSideDryRunner {
	return &K8sServerSideDryRunner{
		dryrunApplier: kubeApplier,
	}
}

// ServerSideApplyDryRun will invoke a kubernetes server-side apply with the given
// obj and the given manager in dryrun mode. Will return the predicted live state
// json as string.
func (kdr *K8sServerSideDryRunner) Run(ctx context.Context, obj *unstructured.Unstructured, manager string) (string, error) {
	//nolint:wrapcheck // trivial function, don't bother wrapping
	return kdr.dryrunApplier.ApplyResource(ctx, obj, cmdutil.DryRunServer, false, false, true, manager)
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

func WithSkipFullNormalize(skip bool) Option {
	return func(o *options) {
		o.skipFullNormalize = skip
	}
}

func WithLogr(log logr.Logger) Option {
	return func(o *options) {
		o.log = log
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

func WithServerSideDiff(ssd bool) Option {
	return func(o *options) {
		o.serverSideDiff = ssd
	}
}

func WithIgnoreMutationWebhook(mw bool) Option {
	return func(o *options) {
		o.ignoreMutationWebhook = mw
	}
}

func WithServerSideDryRunner(ssadr ServerSideDryRunner) Option {
	return func(o *options) {
		o.serverSideDryRunner = ssadr
	}
}

// WithTrustedManagers declares additional field managers that removeWebhookMutation
// should treat as trusted co-owners: fields they manage are yielded to (their live
// value wins) rather than being flagged as drift, the same treatment previously
// reserved for mutation webhooks.
//
// Without this option (trustedManagers == nil), behaviour is unchanged from before
// this option existed: every manager other than the applier itself (see WithManager)
// is trusted. Passing a non-nil slice - including an explicitly empty one - opts into
// stricter behaviour: only the applier and the named managers are trusted, and fields
// owned by anyone else (or by no one at all) are compared normally instead of being
// silently reverted to their live value.
//
// This mirrors the "trusted managers" pattern used to avoid fighting cooperating
// controllers (e.g. crossplane late-init, HPA-owned replicas, mutating webhooks)
// without falling back to trusting literally any manager by default.
func WithTrustedManagers(managers []string) Option {
	return func(o *options) {
		o.trustedManagers = managers
	}
}
