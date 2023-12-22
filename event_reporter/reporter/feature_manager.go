package reporter

import (
	settings_util "github.com/argoproj/argo-cd/v2/util/settings"
	log "github.com/sirupsen/logrus"
	"time"
)

type FeatureManager struct {
	settingsMgr *settings_util.SettingsManager
	shouldRun   bool
}

func NewFeatureManager(settingsMgr *settings_util.SettingsManager) *FeatureManager {
	return &FeatureManager{settingsMgr: settingsMgr}
}

func (f *FeatureManager) setShouldRun() {
	reporterVersion, err := f.settingsMgr.GetCodefreshReporterVersion()
	if err != nil {
		log.Warnf("Failed to get reporter version: %v", err)
		f.shouldRun = false
		return
	}
	f.shouldRun = reporterVersion == string(settings_util.CodefreshV2ReporterVersion)
}

func (f *FeatureManager) Watch() {
	f.setShouldRun()
	// nolint:staticcheck
	tick := time.Tick(5 * time.Second)
	for {
		<-tick
		f.setShouldRun()
	}
}

func (f *FeatureManager) ShouldReporterRun() bool {
	//return f.shouldRun
	return true
}
