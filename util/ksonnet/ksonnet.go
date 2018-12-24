package ksonnet

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/kube"
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

	// ListEnvParams returns list of environment parameters
	ListEnvParams(environment string) ([]*v1alpha1.ComponentParameter, error)

	// SetComponentParams updates component parameter in specified environment.
	SetComponentParams(environment string, component string, param string, value string) error
}

// KsonnetVersion returns the version of ksonnet used when running ksonnet commands
func KsonnetVersion() (string, error) {
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
	return strings.TrimSpace(parts[1]), nil
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
	return k.rootDir
}

// Show generates a concatenated list of Kubernetes manifests in the given environment.
func (k *ksonnetApp) Show(environment string) ([]*unstructured.Unstructured, error) {
	out, err := k.ksCmd("show", environment)
	if err != nil {
		return nil, fmt.Errorf("`ks show` failed: %v", err)
	}
	return kube.SplitYAML(out)
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

// ListEnvParams returns list of environment parameters
func (k *ksonnetApp) ListEnvParams(environment string) ([]*v1alpha1.ComponentParameter, error) {
	log.Infof("listing environment '%s' parameters", environment)
	out, err := k.ksCmd("param", "list", "--output", "json", "--env", environment)
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

	var params []*v1alpha1.ComponentParameter
	for _, ksParam := range ksParams.Data {
		value := strings.Trim(ksParam.Value, `'"`)
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
