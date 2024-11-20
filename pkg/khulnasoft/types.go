package khulnasoft

// VersionSource structure for the versionSource field
type VersionSource struct {
	File     string `json:"file"`
	JsonPath string `json:"jsonPath"`
}

type PromotionTemplate struct {
	VersionSource VersionSource `json:"versionSource"`
}
