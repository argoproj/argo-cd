package featureflag

type featureFlags struct {
	ExampleFeature *featureFlag
}

// New will instantiate a new featureFlags object with all default values.
// Every new feature flag must be initialized in this function or the test
// will fail.
func New() featureFlags {
	return featureFlags{
		ExampleFeature: &featureFlag{
			description: "this is just an example of how a feature flag must be created",
			enabled:     false,
		},
	}
}

type featureFlag struct {
	description string
	enabled     bool
}

func (ff *featureFlag) IsEnabled() bool {
	if ff == nil {
		return false
	}
	return ff.enabled
}

func (ff *featureFlag) Enable() {
	ff.enabled = true
}

func (ff *featureFlag) Disable() {
	ff.enabled = false
}

func (ff *featureFlag) Description() string {
	if ff == nil {
		return ""
	}
	return ff.description
}
