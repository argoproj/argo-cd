package hydrator

import (
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/git"
)

// HydratorCommitMetadata defines the struct used by both Controller and commitServer
// to define the templated commit message and the hydrated manifest
type HydratorCommitMetadata struct {
	RepoURL  string   `json:"repoURL,omitempty"`
	DrySHA   string   `json:"drySha,omitempty"`
	Commands []string `json:"commands,omitempty"`
	Author   string   `json:"author,omitempty"`
	Date     string   `json:"date,omitempty"`
	// Subject is the subject line of the DRY commit message, i.e. `git show --format=%s`.
	Subject string `json:"subject,omitempty"`
	// Body is the body of the DRY commit message, excluding the subject line, i.e. `git show --format=%b`.
	// Known Argocd- trailers with valid values are removed, but all other trailers are kept.
	Body       string                    `json:"body,omitempty"`
	References []appv1.RevisionReference `json:"references,omitempty"`
}

// GetCommitMetadata takes repo, drySha and commitMetadata and returns a HydratorCommitMetadata which is a
// common contract controller and commitServer
func GetCommitMetadata(repoUrl, drySha string, dryCommitMetadata *appv1.RevisionMetadata) (HydratorCommitMetadata, error) { //nolint:revive //FIXME(var-naming)
	author := ""
	message := ""
	date := ""
	var references []appv1.RevisionReference
	if dryCommitMetadata != nil {
		author = dryCommitMetadata.Author
		message = dryCommitMetadata.Message
		if dryCommitMetadata.Date != nil {
			date = dryCommitMetadata.Date.Format(time.RFC3339)
		}
		references = dryCommitMetadata.References
	}

	subject, body, _ := strings.Cut(message, "\n\n")

	_, bodyMinusTrailers := git.GetReferences(log.WithFields(log.Fields{"repo": repoUrl, "revision": drySha}), body)

	hydratorCommitMetadata := HydratorCommitMetadata{
		RepoURL:    repoUrl,
		DrySHA:     drySha,
		Author:     author,
		Subject:    subject,
		Body:       bodyMinusTrailers,
		Date:       date,
		References: references,
	}

	return hydratorCommitMetadata, nil
}
