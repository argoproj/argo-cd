package helm

import (
	"fmt"
	"os/exec"
	"path"
	"strings"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/kube"
)

// Helm provides wrapper functionality around the `helm` command.
type Helm interface {
	// Template returns a list of unstructured objects from a `helm template` command
	Template(name string, valuesFiles []string, params []*argoappv1.ComponentParameter) ([]*unstructured.Unstructured, error)
}

// NewHelmApp create a new wrapper to run commands on the `helm` command-line tool.
func NewHelmApp(path string) Helm {
	return &helm{path: path}
}

type helm struct {
	path string
}

func (h *helm) Template(name string, valuesFiles []string, params []*argoappv1.ComponentParameter) ([]*unstructured.Unstructured, error) {
	args := []string{
		"template", h.path, "--name", name,
	}
	for _, valuesFile := range valuesFiles {
		args = append(args, "-f", path.Join(h.path, valuesFile))
	}
	for _, p := range params {
		args = append(args, "--set", fmt.Sprintf("%s=%s", p.Name, p.Value))
	}
	cmd := exec.Command("helm", args...)
	cmdStr := strings.Join(cmd.Args, " ")
	log.Info(cmdStr)
	outBytes, err := cmd.Output()
	if err != nil {
		exErr, ok := err.(*exec.ExitError)
		if !ok {
			return nil, err
		}
		errOutput := string(exErr.Stderr)
		log.Errorf("`%s` failed: %s", cmdStr, errOutput)
		return nil, fmt.Errorf(strings.TrimSpace(errOutput))
	}
	out := string(outBytes)
	log.Debug(out)
	return kube.SplitYAML(out)
}
