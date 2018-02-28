package ksonnet

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/ksonnet/ksonnet/metadata"
	"github.com/ksonnet/ksonnet/metadata/app"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	diffSeparator = regexp.MustCompile("\\n---")
)

// KsonnetApp represents a ksonnet application directory and provides wrapper functionality around
// the `ks` command.
type KsonnetApp interface {
	// Root is the root path ksonnet application directory
	Root() string

	// Spec is the Ksonnet application spec (app.yaml)
	AppSpec() app.Spec

	// Show returns a list of unstructured objects that would be applied to an environment
	Show(environment string) ([]*unstructured.Unstructured, error)
}

type ksonnetApp struct {
	manager metadata.Manager
	spec    app.Spec
}

func NewKsonnetApp(path string) (KsonnetApp, error) {
	ksApp := ksonnetApp{}
	mgr, err := metadata.Find(path)
	if err != nil {
		return nil, err
	}
	ksApp.manager = mgr
	spec, err := ksApp.manager.AppSpec()
	if err != nil {
		return nil, err
	}
	ksApp.spec = *spec
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

// Spec is the Ksonnet application spec (app.yaml)
func (k *ksonnetApp) AppSpec() app.Spec {
	return k.spec
}

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
