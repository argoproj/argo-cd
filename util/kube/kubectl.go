package kube

import (
	"os"

	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/util/openapi"

	"github.com/argoproj/argo-cd/v3/util/log"

	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/gitops-engine/pkg/utils/tracing"
)

var (
	tracer tracing.Tracer = &tracing.NopTracer{}
	logger logr.Logger    = log.NewLogrusLogger(log.NewWithCurrentConfig())
)

func init() {
	if os.Getenv("ARGOCD_TRACING_ENABLED") == "1" {
		tracer = tracing.NewLoggingTracer(logger)
	}
}

func NewKubectl() kube.Kubectl {
	return &kube.KubectlCmd{Tracer: tracer, Log: logger}
}

func ManageServerSideDiffDryRuns(config *rest.Config, openAPISchema openapi.Resources, onKubectlRun kube.OnKubectlRunFunc) (diff.KubeApplier, func(), error) {
	return kube.ManageServerSideDiffDryRuns(
		config,
		openAPISchema,
		tracer,
		logger,
		onKubectlRun,
	)
}
