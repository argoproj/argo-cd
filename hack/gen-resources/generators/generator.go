package generator

import "github.com/argoproj/argo-cd/v2/hack/gen-resources/util"

var labels = map[string]string{
	"app.kubernetes.io/generated-by": "argocd-generator",
}

type Generator interface {
	Generate(opts *util.GenerateOpts) error
	Clean(opts *util.GenerateOpts) error
}
