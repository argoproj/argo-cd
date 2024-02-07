package codefresh

// VersionSource structure for the versionSource field
type VersionSource struct {
	File     string `json:"file"`
	JsonPath string `json:"jsonPath"`
}

type ApplicationIdentity struct {
	Cluster   string
	Namespace string
	Name      string
}

type ApplicationConfiguration struct {
	VersionSource VersionSource `json:"versionSource"`
}
