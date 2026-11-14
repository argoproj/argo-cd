package generators

type SCMGeneratorWithCustomApiUrl interface { //nolint:revive //FIXME(var-naming)
	CustomApiUrl() string
}
