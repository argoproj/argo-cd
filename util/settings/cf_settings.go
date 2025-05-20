package settings

func (mgr *SettingsManager) GetKustomizeSetNamespaceEnabled() bool {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return false
	}
	kustomizeSetNamespaceEnabled := argoCDCM.Data[kustomizeSetNamespaceEnabledKey]
	if kustomizeSetNamespaceEnabled == "" {
		return false
	}
	return kustomizeSetNamespaceEnabled == "true"
}
