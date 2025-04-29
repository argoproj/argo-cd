package generator

import (
	"context"

	"github.com/argoproj/argo-cd/v3/hack/gen-resources/util"
)

var labels = map[string]string{
	"app.kubernetes.io/generated-by": "argocd-generator",
}

type Generator interface {
	Generate(ctx context.Context, opts *util.GenerateOpts) error
	Clean(ctx context.Context, opts *util.GenerateOpts) error
}
