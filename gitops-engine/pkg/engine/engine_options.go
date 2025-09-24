package engine

import (
	"github.com/go-logr/logr"
	"k8s.io/klog/v2/textlogger"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/gitops-engine/pkg/utils/tracing"
)

type Option func(*options)

type options struct {
	log     logr.Logger
	kubectl kube.Kubectl
}

func applyOptions(opts []Option) options {
	log := textlogger.NewLogger(textlogger.NewConfig())
	o := options{
		log: log,
		kubectl: &kube.KubectlCmd{
			Log:    log,
			Tracer: tracing.NopTracer{},
		},
	}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

func WithLogr(log logr.Logger) Option {
	return func(o *options) {
		o.log = log
		if kcmd, ok := o.kubectl.(*kube.KubectlCmd); ok {
			kcmd.Log = log
		}
	}
}

// SetTracer sets the tracer to use.
func SetTracer(tracer tracing.Tracer) Option {
	return func(o *options) {
		if kcmd, ok := o.kubectl.(*kube.KubectlCmd); ok {
			kcmd.Tracer = tracer
		}
	}
}

// WithKubectl allows to override kubectl wrapper implementation.
func WithKubectl(kubectl kube.Kubectl) Option {
	return func(o *options) {
		o.kubectl = kubectl
	}
}
