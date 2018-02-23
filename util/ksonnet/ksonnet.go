package ksonnet

import (
	"regexp"

	"github.com/ksonnet/ksonnet/metadata"
	"github.com/ksonnet/ksonnet/metadata/app"
)

const (
	appYAMLFile = "app.yaml"
)

var (
	diffSeparator = regexp.MustCompile("\\n---")
)

// KsonnetApp represents a ksonnet application directory
type KsonnetApp struct {
	// Manager abstracts over a ksonnet application's metadata
	Manager metadata.Manager

	// Spec is the Ksonnet application spec (app.yaml)
	Spec app.Spec
}

func NewKsonnetApp(path string) (*KsonnetApp, error) {
	ksApp := KsonnetApp{}
	mgr, err := metadata.Find(path)
	if err != nil {
		return nil, err
	}
	ksApp.Manager = mgr
	spec, err := ksApp.Manager.AppSpec()
	if err != nil {
		return nil, err
	}
	ksApp.Spec = *spec
	return &ksApp, nil
}
