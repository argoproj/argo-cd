package ksonnet

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/diff"
	"github.com/ghodss/yaml"
	"github.com/ksonnet/ksonnet/pkg/app"
	"github.com/ksonnet/ksonnet/pkg/component"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"k8s.io/api/apps/v1beta1"
	"k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	v1ExtBeta1 "k8s.io/api/extensions/v1beta1"
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
	err = cli.UnmarshalLocalFile(filepath.Join(a.Root(), "app.yaml"), &spec)
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
		err = remarshal(&obj)
		if err != nil {
			return nil, err
		}
		objs = append(objs, &obj)
	}
	// TODO(jessesuen): we need to sort objects based on their dependency order of creation
	return objs, nil
}

// remarshal checks resource kind and version and re-marshal using corresponding struct custom marshaller. This ensures that expected resource state is formatter same as actual
// resource state in kubernetes and allows to find differences between actual and target states more accurate.
func remarshal(obj *unstructured.Unstructured) error {
	var newObj interface{}
	switch obj.GetAPIVersion() + ":" + obj.GetKind() {
	case "apps/v1beta1:Deployment":
		newObj = &v1beta1.Deployment{}
	case "apps/v1beta2:Deployment":
		newObj = &v1beta2.Deployment{}
	case "extensions/v1beta1":
		newObj = &v1ExtBeta1.Deployment{}
	case "apps/v1beta1:StatefulSet":
		newObj = &v1beta1.StatefulSet{}
	case "apps/v1beta2:StatefulSet":
		newObj = &v1beta2.StatefulSet{}
	case "v1:Service":
		newObj = &corev1.Service{}
	}
	if newObj != nil {
		oldObj := obj.Object
		data, err := json.Marshal(obj)
		if err != nil {
			return err
		}
		err = json.Unmarshal(data, newObj)
		if err != nil {
			return err
		}
		data, err = json.Marshal(newObj)
		if err != nil {
			return err
		}
		err = json.Unmarshal(data, obj)
		if err != nil {
			return err
		}
		// remove all default values specified by custom formatter
		obj.Object = diff.RemoveMapFields(oldObj, obj.Object)
	}
	return nil
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
