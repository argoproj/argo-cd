package generator

var labels = map[string]string{
	"app.kubernetes.io/generated-by": "argocd-generator",
}

type GenerateOpts struct {
	Samples     int
	GithubToken string
	Namespace   string
}

type Generator interface {
	Generate(opts *GenerateOpts) error
	Clean(opts *GenerateOpts) error
}
