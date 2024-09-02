package repository

import (
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func getChartDetails(chartYAML string) (*v1alpha1.ChartDetails, error) {
	var chart Chart
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
