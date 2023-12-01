package featureflag

// featureFlags This is the main struct where all featureFlags must be
// defined as public attributes following the ExampleFeature pattern.
type featureFlags struct {
	ExampleFeature *featureFlag
}

// New will instantiate a new featureFlags object with all default values.
// Every new feature flag must be initialized in this function or the test
// will fail. Description also needs to be provided or the test will fail.
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

// IsEnabled returns true if the feature flag is enabled.
func (ff *featureFlag) IsEnabled() bool {
	if ff == nil {
		return false
	}
	return ff.enabled
}

// Enable will enable this feature flag.
func (ff *featureFlag) Enable() {
	ff.enabled = true
}

// Disable will disable this feature flag.
func (ff *featureFlag) Disable() {
	ff.enabled = false
}

// Description returns the description of this feature flag.
func (ff *featureFlag) Description() string {
	if ff == nil {
		return ""
	}
	return ff.description
}
