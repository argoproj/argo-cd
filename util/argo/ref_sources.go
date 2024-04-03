package argo

import (
	"context"
	"fmt"
	"regexp"

	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/db"
)

type GetRefSourcesOptions struct {
	Sources    argoappv1.ApplicationSources
	Db         db.ArgoDB
	Revisions  []string
	IsRollback bool
}

// GetRefSources creates a map of ref keys (from the sources' 'ref' fields) to information about the referenced source.
// This function also validates the references use allowed characters and does not define the same ref key more than
// once (which would lead to ambiguous references).
// In case of rollback, this function also updates the targetRevision to the proper revision
func GetRefSources(ctx context.Context, opts GetRefSourcesOptions) (argoappv1.RefTargetRevisionMapping, error) {
	refSources := make(argoappv1.RefTargetRevisionMapping)
	if len(opts.Sources) > 1 {
		// Validate first to avoid unnecessary DB calls.
		refKeys := make(map[string]bool)
		for _, source := range opts.Sources {
			if source.Ref != "" {
				isValidRefKey := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString
				if !isValidRefKey(source.Ref) {
					return nil, fmt.Errorf("sources.ref %s cannot contain any special characters except '_' and '-'", source.Ref)
				}
				refKey := "$" + source.Ref
				if _, ok := refKeys[refKey]; ok {
					return nil, fmt.Errorf("invalid sources: multiple sources had the same `ref` key")
				}
				refKeys[refKey] = true
			}
		}
		// Get Repositories for all sources before generating Manifests
		for i, source := range opts.Sources {
			if source.Ref != "" {
				repo, err := opts.Db.GetRepository(ctx, source.RepoURL)
				if err != nil {
					return nil, fmt.Errorf("failed to get repository %s: %v", source.RepoURL, err)
				}
				refKey := "$" + source.Ref
				revision := source.TargetRevision
				if opts.IsRollback {
					revision = opts.Revisions[i]
				}
				refSources[refKey] = &argoappv1.RefTarget{
					Repo:           *repo,
					TargetRevision: revision,
					Chart:          source.Chart,
				}
			}
		}
	}
	return refSources, nil
}
