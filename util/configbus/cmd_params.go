package configbus

import "os"

// registerCmdParam registers an argocd-cmd-params-cm setting whose Get reads the
// transport env var. Production pods inject cmd-params into env, so env is the
// correct bootstrap source before component Legacy wiring exists.
//
// Caveat: CLI flag overrides that do not also update the process environment are
// invisible here.
func registerCmdParam(name, cmKey, envVar, component string) {
	MustRegister(Setting[string]{
		Name:            name,
		CMKeyExact:      cmKey,
		EnvVar:          envVar,
		Component:       component,
		SourceConfigMap: SourceCmdParamsCM,
		HotReload:       false,
		Get: func(*ResolveContext) (string, error) {
			if envVar == "" {
				return "", nil
			}
			return os.Getenv(envVar), nil
		},
	})
}
