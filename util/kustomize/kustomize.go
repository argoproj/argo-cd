package kustomize

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	certutil "github.com/argoproj/argo-cd/util/cert"
	executil "github.com/argoproj/argo-cd/util/exec"
	"github.com/argoproj/argo-cd/util/git"
)

// represents a Docker image in the format NAME[:TAG].
type Image = string

// Kustomize provides wrapper functionality around the `kustomize` command.
type Kustomize interface {
	// Build returns a list of unstructured objects from a `kustomize build` command and extract supported parameters
	Build(opts *v1alpha1.ApplicationSourceKustomize, kustomizeOptions *v1alpha1.KustomizeOptions) ([]*unstructured.Unstructured, []Image, error)
}

// NewKustomizeApp create a new wrapper to run commands on the `kustomize` command-line tool.
func NewKustomizeApp(path string, creds git.Creds, fromRepo string, binaryPath string) Kustomize {
	return &kustomize{
		path:       path,
		creds:      creds,
		repo:       fromRepo,
		binaryPath: binaryPath,
	}
}

type kustomize struct {
	// path inside the checked out tree
	path string
	// creds structure
	creds git.Creds
	// the Git repository URL where we checked out
	repo string
	// optional kustomize binary path
	binaryPath string
}

func (k *kustomize) getBinaryPath() string {
	if k.binaryPath != "" {
		return k.binaryPath
	}
	return "kustomize"
}

func (k *kustomize) Build(opts *v1alpha1.ApplicationSourceKustomize, kustomizeOptions *v1alpha1.KustomizeOptions) ([]*unstructured.Unstructured, []Image, error) {

	if opts != nil {
		if opts.NamePrefix != "" {
			cmd := exec.Command(k.getBinaryPath(), "edit", "set", "nameprefix", "--", opts.NamePrefix)
			cmd.Dir = k.path
			_, err := executil.Run(cmd)
			if err != nil {
				return nil, nil, err
			}
		}
		if opts.NameSuffix != "" {
			cmd := exec.Command(k.getBinaryPath(), "edit", "set", "namesuffix", "--", opts.NameSuffix)
			cmd.Dir = k.path
			_, err := executil.Run(cmd)
			if err != nil {
				return nil, nil, err
			}
		}
		if len(opts.Images) > 0 {
			// set image postgres=eu.gcr.io/my-project/postgres:latest my-app=my-registry/my-app@sha256:24a0c4b4a4c0eb97a1aabb8e29f18e917d05abfe1b7a7c07857230879ce7d3d3
			// set image node:8.15.0 mysql=mariadb alpine@sha256:24a0c4b4a4c0eb97a1aabb8e29f18e917d05abfe1b7a7c07857230879ce7d3d3
			args := []string{"edit", "set", "image"}
			for _, image := range opts.Images {
				args = append(args, string(image))
			}
			cmd := exec.Command(k.getBinaryPath(), args...)
			cmd.Dir = k.path
			_, err := executil.Run(cmd)
			if err != nil {
				return nil, nil, err
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
			cmd := exec.Command(k.getBinaryPath(), args...)
			cmd.Dir = k.path
			_, err := executil.Run(cmd)
			if err != nil {
				return nil, nil, err
			}
		}

		if len(opts.CommonAnnotations) > 0 {
			//  edit add annotation foo:bar
			args := []string{"edit", "add", "annotation"}
			arg := ""
			for annotationName, annotationValue := range opts.CommonAnnotations {
				if arg != "" {
					arg += ","
				}
				arg += fmt.Sprintf("%s:%s", annotationName, annotationValue)
			}
			args = append(args, arg)
			cmd := exec.Command(k.getBinaryPath(), args...)
			cmd.Dir = k.path
			_, err := executil.Run(cmd)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	var cmd *exec.Cmd
	if kustomizeOptions != nil && kustomizeOptions.BuildOptions != "" {
		params := parseKustomizeBuildOptions(k.path, kustomizeOptions.BuildOptions)
		cmd = exec.Command(k.getBinaryPath(), params...)
	} else {
		cmd = exec.Command(k.getBinaryPath(), "build", k.path)
	}

	cmd.Env = os.Environ()
	closer, environ, err := k.creds.Environ()
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = closer.Close() }()

	// If we were passed a HTTPS URL, make sure that we also check whether there
	// is a custom CA bundle configured for connecting to the server.
	if k.repo != "" && git.IsHTTPSURL(k.repo) {
		parsedURL, err := url.Parse(k.repo)
		if err != nil {
			log.Warnf("Could not parse URL %s: %v", k.repo, err)
		} else {
			caPath, err := certutil.GetCertBundlePathForRepository(parsedURL.Host)
			if err != nil {
				// Some error while getting CA bundle
				log.Warnf("Could not get CA bundle path for %s: %v", parsedURL.Host, err)
			} else if caPath == "" {
				// No cert configured
				log.Debugf("No caCert found for repo %s", parsedURL.Host)
			} else {
				// Make Git use CA bundle
				environ = append(environ, fmt.Sprintf("GIT_SSL_CAINFO=%s", caPath))
			}
		}
	}

	cmd.Env = append(cmd.Env, environ...)
	out, err := executil.Run(cmd)
	if err != nil {
		return nil, nil, err
	}

	objs, err := kube.SplitYAML([]byte(out))
	if err != nil {
		return nil, nil, err
	}

	return objs, getImageParameters(objs), nil
}

func parseKustomizeBuildOptions(path, buildOptions string) []string {
	return append([]string{"build", path}, strings.Split(buildOptions, " ")...)
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

func Version(shortForm bool) (string, error) {
	executable := "kustomize"
	cmdArgs := []string{"version"}
	if shortForm {
		cmdArgs = append(cmdArgs, "--short")
	}
	cmd := exec.Command(executable, cmdArgs...)
	// example version output:
	// long: "{Version:kustomize/v3.8.1 GitCommit:0b359d0ef0272e6545eda0e99aacd63aef99c4d0 BuildDate:2020-07-16T00:58:46Z GoOs:linux GoArch:amd64}"
	// short: "{kustomize/v3.8.1  2020-07-16T00:58:46Z  }"
	version, err := executil.Run(cmd)
	if err != nil {
		return "", fmt.Errorf("could not get kustomize version: %s", err)
	}
	version = strings.TrimSpace(version)
	if shortForm {
		// trim the curly braces
		version = strings.TrimPrefix(version, "{")
		version = strings.TrimSuffix(version, "}")
		version = strings.TrimSpace(version)

		// remove double space in middle
		version = strings.ReplaceAll(version, "  ", " ")

		// remove extra 'kustomize/' before version
		version = strings.TrimPrefix(version, "kustomize/")

	}
	return version, nil
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
