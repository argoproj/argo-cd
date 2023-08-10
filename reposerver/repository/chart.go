package repository

import (
	"fmt"
	"strings"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"sigs.k8s.io/yaml"
)

func getChartDetails(chartYAML string) (*v1alpha1.ChartDetails, error) {
	// see: https://helm.sh/docs/topics/charts/ for more details
	var chart struct {
		Description string `yaml:"description,omitempty"`
		Home        string `yaml:"home,omitempty"`
		Maintainers []struct {
			Name  string `yaml:"name,omitempty"`
			Email string `yaml:"email,omitempty"`
			Url   string `yaml:"url,omitempty"`
		} `yaml:"maintainers,omitempty"`
	}
	err := yaml.Unmarshal([]byte(chartYAML), &chart)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal chart: %w", err)
	}
	var maintainers []string
	for _, maintainer := range chart.Maintainers {
		if maintainer.Email != "" {
			maintainers = append(maintainers, strings.Trim(fmt.Sprintf("%v <%v>", maintainer.Name, maintainer.Email), " "))
		} else {
			maintainers = append(maintainers, fmt.Sprintf("%v", maintainer.Name))
		}
	}
	return &v1alpha1.ChartDetails{
		Description: chart.Description,
		Maintainers: maintainers,
		Home:        chart.Home,
	}, nil
}
