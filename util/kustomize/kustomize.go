package kustomize

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	argoexec "github.com/argoproj/pkg/exec"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/kube"
)

type ImageTag struct {
	Name  string
	Value string
}

func newImageTag(image Image) ImageTag {
	parts := strings.Split(image, ":")
	if len(parts) > 1 {
		return ImageTag{Name: parts[0], Value: parts[1]}
	} else {
		return ImageTag{Name: parts[0], Value: "latest"}
	}
}

// represents a Docker image in the format NAME[:TAG].
type Image = string

// Kustomize provides wrapper functionality around the `kustomize` command.
type Kustomize interface {
	// Build returns a list of unstructured objects from a `kustomize build` command and extract supported parameters
	Build(opts *v1alpha1.ApplicationSourceKustomize) ([]*unstructured.Unstructured, []ImageTag, []Image, error)
}

// NewKustomizeApp create a new wrapper to run commands on the `kustomize` command-line tool.
func NewKustomizeApp(path string, creds *git.Creds) Kustomize {
	return &kustomize{
		path:  path,
		creds: creds,
	}
}

type kustomize struct {
	path  string
	creds *git.Creds
}

func (k *kustomize) Build(opts *v1alpha1.ApplicationSourceKustomize) ([]*unstructured.Unstructured, []ImageTag, []Image, error) {

	version, err := k.getKustomizationVersion()
	if err != nil {
		return nil, nil, nil, err
	}

	commandName := GetCommandName(version)

	if opts != nil {
		if opts.NamePrefix != "" {
			cmd := exec.Command(commandName, "edit", "set", "nameprefix", opts.NamePrefix)
			cmd.Dir = k.path
			_, err := argoexec.RunCommandExt(cmd)
			if err != nil {
				return nil, nil, nil, err
			}
		}

		if len(opts.ImageTags) > 0 {
			if version != 1 {
				log.Warn("ignoring image tags as kustomize is not version 1")
			} else {
				for _, override := range opts.ImageTags {
					cmd := exec.Command(commandName, "edit", "set", "imagetag", fmt.Sprintf("%s:%s", override.Name, override.Value))
					cmd.Dir = k.path
					_, err := argoexec.RunCommandExt(cmd)
					if err != nil {
						return nil, nil, nil, err
					}
				}
			}
		}

		if len(opts.Images) > 0 {
			if version != 2 {
				log.Warn("ignoring images as kustomize is not version 2")
			} else {
				// set image postgres=eu.gcr.io/my-project/postgres:latest my-app=my-registry/my-app@sha256:24a0c4b4a4c0eb97a1aabb8e29f18e917d05abfe1b7a7c07857230879ce7d3d3
				// set image node:8.15.0 mysql=mariadb alpine@sha256:24a0c4b4a4c0eb97a1aabb8e29f18e917d05abfe1b7a7c07857230879ce7d3d3
				args := []string{"edit", "set", "image"}
				args = append(args, opts.Images...)
				cmd := exec.Command(commandName, args...)
				cmd.Dir = k.path
				_, err := argoexec.RunCommandExt(cmd)
				if err != nil {
					return nil, nil, nil, err
				}
			}
		}

		if len(opts.CommonLabels) > 0 {
			//  edit add label foo:bar
			args := []string{"edit", "add", "label"}
			arg := ""
			for labelName, labelValue := range opts.CommonLabels {
				if arg != "" {
					arg += ","
				}
				arg += fmt.Sprintf("%s:%s", labelName, labelValue)
			}
			args = append(args, arg)
			cmd := exec.Command(commandName, args...)
			cmd.Dir = k.path
			_, err := argoexec.RunCommandExt(cmd)
			if err != nil {
				return nil, nil, nil, err
			}
		}
	}

	cmd := exec.Command(commandName, "build", k.path)
	cmd.Env = os.Environ()
	if k.creds != nil {
		cmd.Env = append(cmd.Env, "GIT_ASKPASS=git-ask-pass.sh")
		cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_USERNAME=%s", k.creds.Username))
		cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_PASSWORD=%s", k.creds.Password))
	}

	out, err := argoexec.RunCommandExt(cmd)
	if err != nil {
		return nil, nil, nil, err
	}

	objs, err := kube.SplitYAML(out)
	if err != nil {
		return nil, nil, nil, err
	}

	if version == 1 {
		return objs, getImageTagParameters(objs), nil, nil
	}

	return objs, nil, getImageParameters(objs), nil
}

func GetCommandName(version int) string {
	if version == 1 {
		return "kustomize1"
	}
	return "kustomize"
}

var KustomizationNames = []string{"kustomization.yaml", "kustomization.yml", "Kustomization"}

// kustomization is a file that describes a configuration consumable by kustomize.
func (k *kustomize) findKustomization() (string, error) {
	for _, file := range KustomizationNames {
		kustomization := filepath.Join(k.path, file)
		if _, err := os.Stat(kustomization); err == nil {
			return kustomization, nil
		}
	}
	return "", errors.New("did not find kustomization in " + k.path)
}

func IsKustomization(path string) bool {
	for _, kustomization := range KustomizationNames {
		if path == kustomization {
			return true
		}
	}
	return false
}

type kustomization struct {
	Kind string
}

func (k *kustomize) getKustomizationVersion() (int, error) {

	kustomizationFile, err := k.findKustomization()
	if err != nil {
		return 0, err
	}

	dat, err := ioutil.ReadFile(kustomizationFile)
	if err != nil {
		return 0, err
	}

	var obj kustomization
	err = yaml.Unmarshal(dat, &obj)
	if err != nil {
		return 0, err
	}

	if obj.Kind == "Kustomization" {
		return 2, nil
	}

	return 1, nil
}

func getImageTagParameters(objs []*unstructured.Unstructured) []ImageTag {
	var images []ImageTag
	for _, image := range getImageParameters(objs) {
		images = append(images, newImageTag(image))
	}
	return images
}

func getImageParameters(objs []*unstructured.Unstructured) []Image {
	var images []Image
	for _, obj := range objs {
		images = append(images, getImages(obj.Object)...)
	}
	sort.Slice(images, func(i, j int) bool {
		return i < j
	})
	return images
}

func getImages(object map[string]interface{}) []Image {
	var images []Image
	for k, v := range object {
		if array, ok := v.([]interface{}); ok {
			if k == "containers" || k == "initContainers" {
				for _, obj := range array {
					if mapObj, isMapObj := obj.(map[string]interface{}); isMapObj {
						if image, hasImage := mapObj["image"]; hasImage {
							images = append(images, fmt.Sprintf("%s", image))
						}
					}
				}
			} else {
				for i := range array {
					if mapObj, isMapObj := array[i].(map[string]interface{}); isMapObj {
						images = append(images, getImages(mapObj)...)
					}
				}
			}
		} else if objMap, ok := v.(map[string]interface{}); ok {
			images = append(images, getImages(objMap)...)
		}
	}
	return images
}
