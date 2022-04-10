package kube

import (
	"os"

	"github.com/argoproj/argo-cd/v2/util/log"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/gitops-engine/pkg/utils/tracing"
)

var tracer tracing.Tracer = &tracing.NopTracer{}

func init() {
	if os.Getenv("ARGOCD_TRACING_ENABLED") == "1" {
		tracer = tracing.NewLoggingTracer(log.NewLogrusLogger(log.NewWithCurrentConfig()))
	}
}

func NewKubectl() kube.Kubectl {
	return &kube.KubectlCmd{Tracer: tracer, Log: log.NewLogrusLogger(log.NewWithCurrentConfig())}
}
