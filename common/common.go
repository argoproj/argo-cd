package common

const (
	// MetadataPrefix is the prefix used for our labels and annotations
	MetadataPrefix = "argocd.argoproj.io"
)

var (
	// LabelKeyRepo contains the repository URL
	LabelKeyRepo = MetadataPrefix + "/repo"
)
