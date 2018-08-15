package kustomize

import (
	"github.com/argoproj/pkg/exec"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/util/kube"
)

// Kustomize provides wrapper functionality around the `kustomize` command.
type Kustomize interface {
	// Build returns a list of unstructured objects from a `kustomize build` command
	Build() ([]*unstructured.Unstructured, error)
}

// NewKustomizeApp create a new wrapper to run commands on the `kustomize` command-line tool.
func NewKustomizeApp(path string) Kustomize {
	return &kustomize{path: path}
}

type kustomize struct {
	path string
}

func (k *kustomize) Build() ([]*unstructured.Unstructured, error) {
	out, err := exec.RunCommand("kustomize", "build", k.path)
	if err != nil {
		return nil, err
	}
	return kube.SplitYAML(out)
}
