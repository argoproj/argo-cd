package services

// Legacy* accessors are the sole allowed readers of deprecated ArgoCDService
// fields. InitConfigProvider captures them into StaticFields; product code
// reads via configProvider.

func (a *ArgoCDService) LegacySubmoduleEnabled() bool {
	return a.submoduleEnabled
}

//nolint:staticcheck // SA1019: sole allowed reader of deprecated newFileGlobbingEnabled
func (a *ArgoCDService) LegacyNewFileGlobbingEnabled() bool {
	return a.newFileGlobbingEnabled
}
