package ksonnet

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/ghodss/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	executil "github.com/argoproj/argo-cd/util/exec"
)

// Destination returns the deployment destination for an environment in app spec data
func Destination(data []byte, environment string) (*v1alpha1.ApplicationDestination, error) {
	var appSpec struct {
		Environments map[string]struct {
			Destination v1alpha1.ApplicationDestination
		}
	}
	err := yaml.Unmarshal(data, &appSpec)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal ksonnet spec app.yaml: %v", err)
	}

	envSpec, ok := appSpec.Environments[environment]
	if !ok {
		return nil, fmt.Errorf("environment '%s' does not exist in ksonnet app", environment)
	}

	return &envSpec.Destination, nil
}

// KsonnetApp represents a ksonnet application directory and provides wrapper functionality around
// the `ks` command.
type KsonnetApp interface {
	// Root is the root path ksonnet application directory
	Root() string

	// Show returns a list of unstructured objects that would be applied to an environment
	Show(environment string) ([]*unstructured.Unstructured, error)

	// Destination returns the deployment destination for an environment
	Destination(environment string) (*v1alpha1.ApplicationDestination, error)

	// ListParams returns list of ksonnet parameters
	ListParams(environment string) ([]*v1alpha1.KsonnetParameter, error)

	// SetComponentParams updates component parameter in specified environment.
	SetComponentParams(environment string, component string, param string, value string) error
}

// Version returns the version of ksonnet used when running ksonnet commands
func Version() (string, error) {
	ksApp := ksonnetApp{}
	out, err := ksApp.ksCmd("", "version")
	if err != nil {
		return "", fmt.Errorf("unable to determine ksonnet version: %v", err)
	}
	ksonnetVersionStr := strings.Split(out, "\n")[0]
	parts := strings.SplitN(ksonnetVersionStr, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("unexpected version string format: %s", ksonnetVersionStr)
	}
	version := strings.TrimSpace(parts[1])
	if version[0] != 'v' {
		version = "v" + version
	}
	return version, nil
}

type ksonnetApp struct {
	rootDir string
}

// NewKsonnetApp tries to create a new wrapper to run commands on the `ks` command-line tool.
func NewKsonnetApp(path string) (KsonnetApp, error) {
	ksApp := ksonnetApp{rootDir: path}
	// ensure that the file exists
	if _, err := ksApp.appYamlPath(); err != nil {
		return nil, err
	}
	return &ksApp, nil
}

func (k *ksonnetApp) appYamlPath() (string, error) {
	const appYamlName = "app.yaml"
	p := filepath.Join(k.Root(), appYamlName)
	if _, err := os.Stat(p); err != nil {
		return "", err
	}
	return p, nil
}

func (k *ksonnetApp) ksCmd(args ...string) (string, error) {
	cmd := exec.Command("ks", args...)
	cmd.Dir = k.Root()

	return executil.Run(cmd)
}

func (k *ksonnetApp) Root() string {
	return k.rootDir
}

// Show generates a concatenated list of Kubernetes manifests in the given environment.
func (k *ksonnetApp) Show(environment string) ([]*unstructured.Unstructured, error) {
	out, err := k.ksCmd("show", environment)
	if err != nil {
		return nil, fmt.Errorf("`ks show` failed: %v", err)
	}
	return kube.SplitYAML([]byte(out))
}

// Destination returns the deployment destination for an environment
func (k *ksonnetApp) Destination(environment string) (*v1alpha1.ApplicationDestination, error) {
	p, err := k.appYamlPath()
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}
	return Destination(data, environment)
}

// ListParams returns list of ksonnet parameters
func (k *ksonnetApp) ListParams(environment string) ([]*v1alpha1.KsonnetParameter, error) {
	args := []string{"param", "list", "--output", "json"}
	if environment != "" {
		args = append(args, "--env", environment)
	}
	out, err := k.ksCmd(args...)
	if err != nil {
		return nil, err
	}
	// Auxiliary data to hold unmarshaled JSON output, which may use different field names
	var ksParams struct {
		Data []struct {
			Component string `json:"component"`
			Key       string `json:"param"`
			Value     string `json:"value"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &ksParams); err != nil {
		return nil, err
	}
	var params []*v1alpha1.KsonnetParameter
	for _, ksParam := range ksParams.Data {
		value := strings.Trim(ksParam.Value, `'"`)
		params = append(params, &v1alpha1.KsonnetParameter{
			Component: ksParam.Component,
			Name:      ksParam.Key,
			Value:     value,
		})
	}
	return params, nil
}

// SetComponentParams updates component parameter in specified environment.
func (k *ksonnetApp) SetComponentParams(environment string, component string, param string, value string) error {
	_, err := k.ksCmd("param", "set", component, param, value, "--env", environment)
	return err
}
