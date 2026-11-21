package commit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	imagev1 "github.com/opencontainers/image-spec/specs-go/v1"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/commitserver/apiclient"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/hydrator"
	"github.com/argoproj/argo-cd/v3/util/io"
	"github.com/argoproj/argo-cd/v3/util/io/files"
	"github.com/argoproj/argo-cd/v3/util/oci"
)

const (
	// OCIArtifactType is the artifact type of the OCI artifacts holding hydrated manifests.
	OCIArtifactType = "application/vnd.argoproj.argocd.hydrated-manifests.v1"
	// OCIManifestsDigestAnnotation is the annotation on the OCI manifest holding a digest of the hydrated manifests
	// in the artifact. It is how the hydrator recognizes that a tag already holds the manifests it is about to push.
	OCIManifestsDigestAnnotation = "io.argoproj.argocd.hydrator.manifests-digest"
)

// newOCIPusher is the default factory for the client pushing hydrated manifests to an OCI repository.
func newOCIPusher(repo *v1alpha1.Repository) (oci.Pusher, error) {
	return oci.NewPusher(repo.Repo, repo.GetOCICreds(), repo.Proxy, repo.NoProxy)
}

func isOCIRepo(repoURL string) bool {
	return strings.HasPrefix(repoURL, "oci://")
}

// handleOCICommitRequest handles a commit request for an OCI repository. It writes the manifests to a scratch
// directory and pushes its contents as a single-layer artifact to the tag given as the target branch, creating the tag
// if it does not exist. Nothing is pushed if the tag already holds the same manifests. It returns the digest of the
// OCI manifest the tag points to.
//
// Pushing a tag replaces the whole artifact, so the request must contain every path that is hydrated to the tag.
func (s *Service) handleOCICommitRequest(ctx context.Context, logCtx *log.Entry, r *apiclient.CommitHydratedManifestsRequest) (string, error) {
	tag := r.TargetBranch
	if tag == "" {
		return "", errors.New("target tag is required")
	}

	// Serialize requests for the same tag so that the comparison with the manifests the tag currently holds and
	// the push that follows are not interleaved with another request. Per-replica only.
	tagKey := r.Repo.Repo + "\x00" + tag
	s.branchLock.Lock(tagKey)
	defer s.branchLock.Unlock(tagKey)

	logCtx = logCtx.WithField("repo", r.Repo.Repo)
	if s.signingConfig != nil {
		logCtx.Warn("Signing of hydrated commits is enabled, but it does not apply to OCI artifacts. The artifact is pushed unsigned.")
	}

	dirPath, err := files.CreateTempDir("/tmp/_commit-service")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(dirPath); err != nil {
			logCtx.WithError(err).Error("failed to cleanup temp dir")
		}
	}()

	root, err := os.OpenRoot(dirPath)
	if err != nil {
		return "", fmt.Errorf("failed to open root dir: %w", err)
	}
	defer io.Close(root)

	logCtx.Debug("Writing manifests")
	manifestsDigest, err := writeForOCI(root, r)
	if err != nil {
		return "", fmt.Errorf("failed to write manifests: %w", err)
	}

	pusher, err := s.newOCIPusher(r.Repo)
	if err != nil {
		return "", fmt.Errorf("failed to create OCI client: %w", err)
	}

	logCtx.Debugf("Resolving tag %s", tag)
	currentDigest, currentAnnotations, err := pusher.ResolveTag(ctx, tag)
	switch {
	case err == nil:
		if currentAnnotations[OCIManifestsDigestAnnotation] == manifestsDigest {
			// Manifests did not change, so we don't need to push a new artifact.
			logCtx.Debugf("tag %s already holds the manifests of dry sha %s", tag, r.DrySha)
			return currentDigest, nil
		}
	case !errors.Is(err, oci.ErrTagNotFound):
		return "", fmt.Errorf("failed to resolve tag: %w", err)
	}

	logCtx.Debug("Pushing artifact")
	digest, err := pusher.PushDirectory(ctx, dirPath, tag, oci.PushOptions{
		ArtifactType: OCIArtifactType,
		Annotations:  ociAnnotations(r, manifestsDigest),
	})
	if err != nil {
		return "", fmt.Errorf("failed to push: %w", err)
	}
	return digest, nil
}

// writeForOCI writes the manifests, hydrator.metadata, and README.md files for each path of the request, and a
// root-level hydrator.metadata file describing the dry commit. It returns a digest of the written manifests.
func writeForOCI(root *os.Root, r *apiclient.CommitHydratedManifestsRequest) (string, error) {
	commitMetadata, err := hydrator.GetCommitMetadata(r.DryRepoURL, r.DrySha, r.DryCommitMetadata)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve hydrator metadata: %w", err)
	}
	err = writeMetadata(root, "", commitMetadata)
	if err != nil {
		return "", fmt.Errorf("failed to write top-level hydrator metadata: %w", err)
	}

	manifestPaths := make([]string, 0, len(r.Paths))
	for _, p := range r.Paths {
		hydratePath := filepath.Clean(p.Path)
		if hydratePath == "." || hydratePath == string(filepath.Separator) {
			hydratePath = ""
		}
		if slices.Contains(manifestPaths, hydratePath) {
			return "", fmt.Errorf("more than one application is hydrated to path %q", p.Path)
		}
		manifestPaths = append(manifestPaths, hydratePath)

		pathMetadata := hydrator.HydratorCommitMetadata{
			Commands: p.Commands,
			DrySHA:   r.DrySha,
			RepoURL:  r.DryRepoURL,
		}
		if hydratePath == "" {
			// The metadata of an application at the root of the artifact shares its file with the metadata of the
			// dry commit, so the file has to hold both.
			pathMetadata = commitMetadata
			pathMetadata.Commands = p.Commands
		} else {
			err = root.MkdirAll(hydratePath, 0o755)
			if err != nil {
				return "", fmt.Errorf("failed to create path: %w", err)
			}
		}

		err = writeManifests(root, hydratePath, p.Manifests)
		if err != nil {
			return "", fmt.Errorf("failed to write manifests: %w", err)
		}
		err = writeMetadata(root, hydratePath, pathMetadata)
		if err != nil {
			return "", fmt.Errorf("failed to write hydrator metadata: %w", err)
		}
		err = writeReadme(root, hydratePath, pathMetadata, r.ReadmeMessage)
		if err != nil {
			return "", fmt.Errorf("failed to write readme: %w", err)
		}
	}

	// The digest covers nothing but the paths and their manifests. The other files describe the dry commit and would
	// make every dry commit look like a change.
	slices.Sort(manifestPaths)
	hash := sha256.New()
	for _, manifestPath := range manifestPaths {
		manifest, err := root.ReadFile(filepath.Join(manifestPath, ManifestYaml))
		if err != nil {
			return "", fmt.Errorf("failed to read manifest: %w", err)
		}
		// Length prefixes keep the boundaries between paths and manifests unambiguous.
		fmt.Fprintf(hash, "%d:%s%d:", len(manifestPath), manifestPath, len(manifest))
		hash.Write(manifest)
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

// ociAnnotations returns the annotations for the OCI manifest of the pushed artifact.
func ociAnnotations(r *apiclient.CommitHydratedManifestsRequest, manifestsDigest string) map[string]string {
	annotations := map[string]string{
		OCIManifestsDigestAnnotation: manifestsDigest,
		imagev1.AnnotationRevision:   r.DrySha,
	}
	if r.DryRepoURL != "" {
		annotations[imagev1.AnnotationSource] = r.DryRepoURL
	}
	if r.DryCommitMetadata != nil {
		if r.DryCommitMetadata.Author != "" {
			annotations[imagev1.AnnotationAuthors] = r.DryCommitMetadata.Author
		}
		// Use the date of the dry commit so that pushing the same content again yields the same artifact.
		if r.DryCommitMetadata.Date != nil && !r.DryCommitMetadata.Date.IsZero() {
			annotations[imagev1.AnnotationCreated] = r.DryCommitMetadata.Date.UTC().Format(time.RFC3339)
		}
	}
	return annotations
}
