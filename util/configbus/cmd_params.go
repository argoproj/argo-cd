package configbus

// registerCmdParam registers a coverage-only argocd-cmd-params-cm setting.
// Used by cmd_params_generated.go.
func registerCmdParam(name, cmKey, envVar, component string) {
	MustRegister(Setting[string]{
		Name:            name,
		CMKeyExact:      cmKey,
		EnvVar:          envVar,
		Component:       component,
		SourceConfigMap: SourceCmdParamsCM,
		HotReload:       false,
		Get:             panicGet[string](name),
	})
}
