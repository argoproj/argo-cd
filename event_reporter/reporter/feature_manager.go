package reporter

import (
	settings_util "github.com/argoproj/argo-cd/v2/util/settings"
)

type FeatureManager struct {
	settingsMgr *settings_util.SettingsManager
}

func NewFeatureManager(settingsMgr *settings_util.SettingsManager) *FeatureManager {
	return &FeatureManager{settingsMgr: settingsMgr}
}
