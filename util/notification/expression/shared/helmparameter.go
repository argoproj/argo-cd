package shared

type HelmParameter struct {
	// Name is the name of the helm parameter
	Name string
	// Value is the value for the helm parameter
	Value string
	// ForceString determines whether to tell Helm to interpret booleans and numbers as strings
	ForceString bool
}
