package helm

import (
	"fmt"
	"io/ioutil"
	"path"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	// HelmV2 represents helm V2 specific settings
	HelmV2 = HelmVer{
		binaryName:           "helm2",
		templateNameArg:      "--name",
		kubeVersionSupported: true,
		showCommand:          "inspect",
		pullCommand:          "fetch",
		initSupported:        true,
	}
	// HelmV3 represents helm V3 specific settings
	HelmV3 = HelmVer{
		binaryName:           "helm",
		templateNameArg:      "--name-template",
		kubeVersionSupported: false,
		showCommand:          "show",
		pullCommand:          "pull",
		initSupported:        false,
	}
)

func getHelmVersion(chartPath string) (*HelmVer, error) {
	data, err := ioutil.ReadFile(path.Join(chartPath, "Chart.yaml"))
	if err != nil {
		return nil, err
	}
	obj := make(map[string]interface{})
	err = yaml.Unmarshal(data, &obj)
	if err != nil {
		return nil, err
	}
	version, _, _ := unstructured.NestedString(obj, "apiVersion")
	switch version {
	case "", "v1":
		return &HelmV2, nil
	case "v2":
		return &HelmV3, nil

	}
	return nil, fmt.Errorf("helm chart version '%s' is not supported", version)
}

// HelmVer contains Helm version specific settings such as helm binary and command names
type HelmVer struct {
	binaryName           string
	initSupported        bool
	templateNameArg      string
	showCommand          string
	pullCommand          string
	kubeVersionSupported bool
}
