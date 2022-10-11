package image

import (
	imagepkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/image"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func image(f *v1.ConfigFile) *imagepkg.Image {
	return &imagepkg.Image{
		Created: &metav1.Time{
			Time: f.Created.Time,
		},
		Author: f.Author,
		Config: config(f.Config),
	}
}

func config(c v1.Config) *imagepkg.Config {
	return &imagepkg.Config{
		Command:      c.Cmd,
		Entrypoint:   c.Entrypoint,
		Env:          c.Env,
		WorkingDir:   c.WorkingDir,
		ExposedPorts: exposedPorts(c.ExposedPorts),
		Labels:       c.Labels,
	}
}

func exposedPorts(ports map[string]struct{}) map[string]*imagepkg.Port {
	out := make(map[string]*imagepkg.Port)
	for key := range ports {
		out[key] = &imagepkg.Port{}
	}
	return out
}
