package kustomize

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/pkg/errors"
	"github.com/prometheus/common/log"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/kube"
	argoexec "github.com/argoproj/pkg/exec"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Kustomize provides wrapper functionality around the `kustomize` command.
type Kustomize interface {
	// Build returns a list of unstructured objects from a `kustomize build` command and extract supported parameters
	Build(opts KustomizeBuildOpts, overrides []*v1alpha1.ComponentParameter) ([]*unstructured.Unstructured, []*v1alpha1.ComponentParameter, error)
}

// NewKustomizeApp create a new wrapper to run commands on the `kustomize` command-line tool.
func NewKustomizeApp(path string) Kustomize {
	return &kustomize{path: path}
}

type kustomize struct {
	path string
}

// KustomizeBuildOpts are options to a `kustomize build` command
type KustomizeBuildOpts struct {
	// NamePrefix will run `kustomize edit set nameprefix` during manifest generation
	NamePrefix string
}

func (k *kustomize) Build(opts KustomizeBuildOpts, overrides []*v1alpha1.ComponentParameter) ([]*unstructured.Unstructured, []*v1alpha1.ComponentParameter, error) {

	version, err := k.getKustomizationVersion()
	if err != nil {
		return nil, nil, err
	}

	log.Infof("using kustomize version=%d", version)

	commandName := GetCommandName(version)

	log.Infof("using kustomize binary=%s", commandName)

	if opts.NamePrefix != "" {
		cmd := exec.Command(commandName, "edit", "set", "nameprefix", opts.NamePrefix)
		cmd.Dir = k.path
		_, err := argoexec.RunCommandExt(cmd)
		if err != nil {
			return nil, nil, err
		}
	}

	if version == 1 {
		for _, override := range overrides {
			cmd := exec.Command(commandName, "edit", "set", "imagetag", fmt.Sprintf("%s:%s", override.Name, override.Value))
			cmd.Dir = k.path
			_, err := argoexec.RunCommandExt(cmd)
			if err != nil {
				return nil, nil, err
			}
		}
	} else if len(overrides) > 0 {
		log.Info("ignoring overrides as kustomize is not version 1")
	}

	out, err := argoexec.RunCommand(commandName, "build", k.path)
	if err != nil {
		return nil, nil, err
	}

	objs, err := kube.SplitYAML(out)
	if err != nil {
		return nil, nil, err
	}

	parameters := k.getParameters(objs)

	return objs, parameters, nil
}

func (k *kustomize) getParameters(objs []*unstructured.Unstructured) []*v1alpha1.ComponentParameter {
	version, _ := k.getKustomizationVersion() // cannot be an error at this line
	if version == 1 {
		return getImageParameters(objs)
	} else {
		return []*v1alpha1.ComponentParameter{}
	}
}

func GetCommandName(version int) string {
	if version == 1 {
		return "kustomize"
	} else {
		return "kustomize" + strconv.Itoa(version)
	}
}

var KustomizationNames = []string{"kustomization.yaml", "kustomization.yml", "Kustomization"}

// kustomization is a file that describes a configuration consumable by kustomize.
func (k *kustomize) findKustomization() (string, error) {
	for _, file := range KustomizationNames {
		kustomization := filepath.Join(k.path, file)
		log.Infof("path=%s, file=%s", k.path, file)
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

	log.Infof("using kustomization=%s", kustomizationFile)

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

func getImageParameters(objs []*unstructured.Unstructured) []*v1alpha1.ComponentParameter {
	images := make(map[string]string)
	for _, obj := range objs {
		for _, img := range getImages(obj.Object) {
			parts := strings.Split(img, ":")
			if len(parts) > 1 {
				images[parts[0]] = parts[1]
			} else {
				images[img] = "latest"
			}
		}
	}
	var params []*v1alpha1.ComponentParameter
	for img, version := range images {
		params = append(params, &v1alpha1.ComponentParameter{
			Component: "imagetag",
			Name:      img,
			Value:     version,
		})
	}
	return params
}

func getImages(object map[string]interface{}) []string {
	var images []string
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
