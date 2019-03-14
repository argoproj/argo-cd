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
	yaml "gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/kube"
)

// Kustomize provides wrapper functionality around the `kustomize` command.
type Kustomize interface {
	// Build returns a list of unstructured objects from a `kustomize build` command and extract supported parameters
	Build(opts *v1alpha1.ApplicationSourceKustomize) ([]*unstructured.Unstructured, []*v1alpha1.KustomizeImageTag, error)
}

type GitCredentials struct {
	Username string
	Password string
}

// NewKustomizeApp create a new wrapper to run commands on the `kustomize` command-line tool.
func NewKustomizeApp(path string, creds *GitCredentials) Kustomize {
	return &kustomize{
		path:  path,
		creds: creds,
	}
}

type kustomize struct {
	path  string
	creds *GitCredentials
}

func (k *kustomize) Build(opts *v1alpha1.ApplicationSourceKustomize) ([]*unstructured.Unstructured, []*v1alpha1.KustomizeImageTag, error) {

	version, err := k.getKustomizationVersion()
	if err != nil {
		return nil, nil, err
	}

	commandName := GetCommandName(version)

	if opts != nil {
		if opts.NamePrefix != "" {
			cmd := exec.Command(commandName, "edit", "set", "nameprefix", opts.NamePrefix)
			cmd.Dir = k.path
			_, err := argoexec.RunCommandExt(cmd)
			if err != nil {
				return nil, nil, err
			}
		}

		if version == 1 {
			for _, override := range opts.ImageTags {
				cmd := exec.Command(commandName, "edit", "set", "imagetag", fmt.Sprintf("%s:%s", override.Name, override.Value))
				cmd.Dir = k.path
				_, err := argoexec.RunCommandExt(cmd)
				if err != nil {
					return nil, nil, err
				}
			}
		} else if len(opts.ImageTags) > 0 {
			log.Info("ignoring overrides as kustomize is not version 1")
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
		return nil, nil, err
	}

	objs, err := kube.SplitYAML(out)
	if err != nil {
		return nil, nil, err
	}

	parameters := k.getParameters(objs)

	return objs, parameters, nil
}

func (k *kustomize) getParameters(objs []*unstructured.Unstructured) []*v1alpha1.KustomizeImageTag {
	version, _ := k.getKustomizationVersion() // cannot be an error at this line
	if version == 1 {
		return getImageParameters(objs)
	} else {
		return []*v1alpha1.KustomizeImageTag{}
	}
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

func getImageParameters(objs []*unstructured.Unstructured) []*v1alpha1.KustomizeImageTag {
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
	var params []*v1alpha1.KustomizeImageTag
	for img, version := range images {
		params = append(params, &v1alpha1.KustomizeImageTag{
			Name:  img,
			Value: version,
		})
	}
	sort.Slice(params, func(i, j int) bool {
		return params[i].Name < params[j].Name
	})
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
