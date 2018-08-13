package ksonnet

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ksonnet/ksonnet/pkg/app"
	"github.com/ksonnet/ksonnet/pkg/component"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/config"
	"github.com/argoproj/argo-cd/util/kube"
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

// KsonnetVersion returns the version of ksonnet used when running ksonnet commands
func KsonnetVersion() (string, error) {
	cmd := exec.Command("ks", "version")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("unable to determine ksonnet version: %v", err)
	}
	ksonnetVersionStr := strings.Split(string(out), "\n")[0]
	parts := strings.SplitN(ksonnetVersionStr, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("unexpected version string format: %s", ksonnetVersionStr)
	}
	return strings.TrimSpace(parts[1]), nil
}

type ksonnetApp struct {
	app  app.App
	spec app.Spec
}

// NewKsonnetApp tries to create a new wrapper to run commands on the `ks` command-line tool.
func NewKsonnetApp(path string) (KsonnetApp, error) {
	ksApp := ksonnetApp{}
	a, err := app.Load(afero.NewOsFs(), path, false)
	if err != nil {
		return nil, err
	}
	ksApp.app = a

	var spec app.Spec
	err = config.UnmarshalLocalFile(filepath.Join(a.Root(), "app.yaml"), &spec)
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
	outBytes, err := cmd.Output()
	if err != nil {
		exErr, ok := err.(*exec.ExitError)
		if !ok {
			return "", err
		}
		errOutput := string(exErr.Stderr)
		log.Errorf("`%s` failed: %s", cmdStr, errOutput)
		return "", fmt.Errorf(strings.TrimSpace(errOutput))
	}
	out := string(outBytes)
	log.Debug(out)
	return out, nil
}

func (k *ksonnetApp) Root() string {
	return k.app.Root()
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
		return nil, fmt.Errorf("`ks show` failed: %v", err)
	}
	data, err := kube.SplitYAML(out)
	if err != nil {
		for _, d := range data {
			kube.UnsetLabel(d, "ksonnet.io/component")
		}
	}
	return data, err
}

// ListEnvParams returns list of environment parameters
func (k *ksonnetApp) ListEnvParams(environment string) ([]*v1alpha1.ComponentParameter, error) {
	log.Infof("listing environment '%s' parameters", environment)
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
