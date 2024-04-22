package commit

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"time"
)

/**
The commit package provides a way for the controller to push manifests to git.
*/

type Service interface {
	Commit(ManifestsRequest) ManifestsResponse
}

type ManifestsRequest struct {
	RepoURL       string
	TargetBranch  string
	DrySHA        string
	CommitAuthor  string
	CommitMessage string
	CommitTime    time.Time
	PathDetails
}

type PathDetails struct {
	Path      string
	Manifests []ManifestDetails
	ReadmeDetails
}

type ManifestDetails struct {
	Manifest unstructured.Unstructured
}

type ReadmeDetails struct {
}

type ManifestsResponse struct {
	RequestId string
}
