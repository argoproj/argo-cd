package ksonnet

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/ghodss/yaml"
	"github.com/ksonnet/ksonnet/metadata"
	"github.com/ksonnet/ksonnet/pkg/app"
	"github.com/ksonnet/ksonnet/pkg/component"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	diffSeparator = regexp.MustCompile(`\n---`)
)

// KsonnetApp represents a ksonnet application directory and provides wrapper functionality around
// the `ks` command.
type KsonnetApp interface {
	// Root is the root path ksonnet application directory
	Root() string

	// App is the Ksonnet application
	App() app.App

	// Spec is the Ksonnet application spec
	Spec() *app.Spec

	// Show returns a list of unstructured objects that would be applied to an environment
	Show(environment string) ([]*unstructured.Unstructured, error)

	// ListEnvParams returns list of environment parameters
	ListEnvParams(environment string) ([]*v1alpha1.ComponentParameter, error)

	// SetComponentParams updates component parameter in specified environment.
	SetComponentParams(environment string, component string, param string, value string) error
}

type ksonnetApp struct {
	manager metadata.Manager
	app     app.App
	spec    app.Spec
}

// NewKsonnetApp tries to create a new wrapper to run commands on the `ks` command-line tool.
func NewKsonnetApp(path string) (KsonnetApp, error) {
	ksApp := ksonnetApp{}
	mgr, err := metadata.Find(path)
	if err != nil {
		return nil, err
	}
	ksApp.manager = mgr
	a, err := ksApp.manager.App()
	if err != nil {
		return nil, err
	}
	ksApp.app = a

	var spec app.Spec
	err = cli.UnmarshalLocalFile(filepath.Join(ksApp.manager.Root(), "app.yaml"), &spec)
	if err != nil {
		return nil, err
	}
	ksApp.spec = spec
	return &ksApp, nil
}

func (k *ksonnetApp) ksCmd(args ...string) (string, error) {
	cmd := exec.Command("ks", args...)
	cmd.Dir = k.Root()

	cmdStr := strings.Join(cmd.Args, " ")
	log.Debug(cmdStr)
	out, err := cmd.Output()
	if err != nil {
		exErr, ok := err.(*exec.ExitError)
		if !ok {
			return "", err
		}
		errOutput := string(exErr.Stderr)
		log.Errorf("`%s` failed: %s", cmdStr, errOutput)
		return "", fmt.Errorf(strings.TrimSpace(errOutput))
	}
	return string(out), nil
}

func (k *ksonnetApp) Root() string {
	return k.manager.Root()
}

// App is the Ksonnet application
func (k *ksonnetApp) App() app.App {
	return k.app
}

// Spec is the Ksonnet application spec
func (k *ksonnetApp) Spec() *app.Spec {
	return &k.spec
}

// Show generates a concatenated list of Kubernetes manifests in the given environment.
func (k *ksonnetApp) Show(environment string) ([]*unstructured.Unstructured, error) {
	out, err := k.ksCmd("show", environment)
	if err != nil {
		return nil, err
	}
	parts := diffSeparator.Split(out, -1)
	objs := make([]*unstructured.Unstructured, 0)
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		var obj unstructured.Unstructured
		err = yaml.Unmarshal([]byte(part), &obj)
		if err != nil {
			return nil, fmt.Errorf("Failed to unmarshal manifest from `ks show`")
		}
		objs = append(objs, &obj)
	}
	// TODO(jessesuen): we need to sort objects based on their dependency order of creation
	return objs, nil
}

// ListEnvParams returns list of environment parameters
func (k *ksonnetApp) ListEnvParams(environment string) ([]*v1alpha1.ComponentParameter, error) {
	mod, err := component.DefaultManager.Module(k.app, "")
	if err != nil {
		return nil, err
	}
	ksParams, err := mod.Params(environment)
	if err != nil {
		return nil, err
	}
	var params []*v1alpha1.ComponentParameter
	for _, ksParam := range ksParams {
		value, err := strconv.Unquote(ksParam.Value)
		if err != nil {
			value = ksParam.Value
		}
		componentParam := v1alpha1.ComponentParameter{
			Component: ksParam.Component,
			Name:      ksParam.Key,
			Value:     value,
		}
		params = append(params, &componentParam)
	}
	return params, nil
}

// SetComponentParams updates component parameter in specified environment.
func (k *ksonnetApp) SetComponentParams(environment string, component string, param string, value string) error {
	_, err := k.ksCmd("param", "set", component, param, value, "--env", environment)
	return err
}
