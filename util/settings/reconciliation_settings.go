package settings

import (
	"github.com/argoproj/argo-cd/engine/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/engine/resource"
)

type StaticReconciliationSettings struct {
	AppInstanceLabelKey     string
	ResourcesFilter         resource.ResourcesFilter
	ResourceOverrides       map[string]v1alpha1.ResourceOverride
	ConfigManagementPlugins []v1alpha1.ConfigManagementPlugin
	KustomizeBuildOptions   string
}

func (s *StaticReconciliationSettings) GetAppInstanceLabelKey() (string, error) {
	return s.AppInstanceLabelKey, nil
}

func (s *StaticReconciliationSettings) GetResourcesFilter() (*resource.ResourcesFilter, error) {
	return &s.ResourcesFilter, nil
}

func (s *StaticReconciliationSettings) GetResourceOverrides() (map[string]v1alpha1.ResourceOverride, error) {
	if s.ResourceOverrides == nil {
		return make(map[string]v1alpha1.ResourceOverride), nil
	}
	return s.ResourceOverrides, nil
}

func (s *StaticReconciliationSettings) GetConfigManagementPlugins() ([]v1alpha1.ConfigManagementPlugin, error) {
	return s.ConfigManagementPlugins, nil
}
func (s *StaticReconciliationSettings) GetKustomizeBuildOptions() (string, error) {
	return s.KustomizeBuildOptions, nil
}

func (s *StaticReconciliationSettings) Subscribe(subCh chan<- bool) {

}

func (s *StaticReconciliationSettings) Unsubscribe(subCh chan<- bool) {

}
