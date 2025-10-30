package commit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/commitserver/apiclient"
	"github.com/argoproj/argo-cd/v3/common"
	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/git"
	"github.com/argoproj/argo-cd/v3/util/hydrator"
	"github.com/argoproj/argo-cd/v3/util/io"
)

var sprigFuncMap = sprig.GenericFuncMap() // a singleton for better performance

const gitAttributesContents = `*/README.md linguist-generated=true
*/hydrator.metadata linguist-generated=true`

func init() {
	// Avoid allowing the user to learn things about the environment.
	delete(sprigFuncMap, "env")
	delete(sprigFuncMap, "expandenv")
	delete(sprigFuncMap, "getHostByName")
}

// WriteForPaths writes the manifests, hydrator.metadata, and README.md files for each path in the provided paths. It
// also writes a root-level hydrator.metadata file containing the repo URL and dry SHA.
func WriteForPaths(root *os.Root, repoUrl, drySha string, dryCommitMetadata *appv1.RevisionMetadata, paths []*apiclient.PathDetails, gitClient git.Client) (bool, error) { //nolint:revive //FIXME(var-naming)
	hydratorMetadata, err := hydrator.GetCommitMetadata(repoUrl, drySha, dryCommitMetadata)
	if err != nil {
		return false, fmt.Errorf("failed to retrieve hydrator metadata: %w", err)
	}

	// Write the top-level readme.
	err = writeMetadata(root, "", hydratorMetadata)
	if err != nil {
		return false, fmt.Errorf("failed to write top-level hydrator metadata: %w", err)
	}

	// Write .gitattributes
	err = writeGitAttributes(root)
	if err != nil {
		return false, fmt.Errorf("failed to write git attributes: %w", err)
	}
	var atleastOneManifestChanged bool
	for _, p := range paths {
		hydratePath := p.Path
		if hydratePath == "." {
			hydratePath = ""
		}

		// Only create directory if path is not empty (root directory case)
		if hydratePath != "" {
			err = root.MkdirAll(hydratePath, 0o755)
			if err != nil {
				return false, fmt.Errorf("failed to create path: %w", err)
			}
		}

		// Write the manifests
		err := writeManifests(root, hydratePath, p.Manifests)
		if err != nil {
			return false, fmt.Errorf("failed to write manifests: %w", err)
		}
		changed, err := hasManifestChanged(hydratePath, gitClient)
		if err != nil {
			return false, fmt.Errorf("failed to check if anything changed on the manifest: %w", err)
		}

		if !changed {
			err = deleteManifest(root, hydratePath)
			if err != nil {
				return false, fmt.Errorf("failed to delete the un-changed manifest: %w", err)
			}
			continue
		}
		//  If any manifest has changed, signal that a commit should occur. If none have changed, skip committing.
		atleastOneManifestChanged = changed

		// Write hydrator.metadata containing information about the hydration process.
		hydratorMetadata := hydrator.HydratorCommitMetadata{
			Commands: p.Commands,
			DrySHA:   drySha,
			RepoURL:  repoUrl,
		}
		err = writeMetadata(root, hydratePath, hydratorMetadata)
		if err != nil {
			return false, fmt.Errorf("failed to write hydrator metadata: %w", err)
		}

		// Write README
		err = writeReadme(root, hydratePath, hydratorMetadata)
		if err != nil {
			return false, fmt.Errorf("failed to write readme: %w", err)
		}
	}
	// if no manifest changes then skip commit
	if !atleastOneManifestChanged {
		return false, nil
	}
	return atleastOneManifestChanged, nil
}

// writeMetadata writes the metadata to the hydrator.metadata file.
func writeMetadata(root *os.Root, dirPath string, metadata hydrator.HydratorCommitMetadata) error {
	hydratorMetadataPath := filepath.Join(dirPath, "hydrator.metadata")
	f, err := root.Create(hydratorMetadataPath)
	if err != nil {
		return fmt.Errorf("failed to create hydrator metadata file: %w", err)
	}
	defer io.Close(f)
	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	// We don't need to escape HTML, because we're not embedding this JSON in HTML.
	e.SetEscapeHTML(false)
	err = e.Encode(metadata)
	if err != nil {
		return fmt.Errorf("failed to encode hydrator metadata: %w", err)
	}
	return nil
}

// writeReadme writes the readme to the README.md file.
func writeReadme(root *os.Root, dirPath string, metadata hydrator.HydratorCommitMetadata) error {
	readmeTemplate, err := template.New("readme").Funcs(sprigFuncMap).Parse(manifestHydrationReadmeTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse readme template: %w", err)
	}
	// Create writer to template into
	// No need to use SecureJoin here, as the path is already sanitized.
	readmePath := filepath.Join(dirPath, "README.md")
	readmeFile, err := root.Create(readmePath)
	if err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create README file: %w", err)
	}
	err = readmeTemplate.Execute(readmeFile, metadata)
	closeErr := readmeFile.Close()
	if closeErr != nil {
		log.WithError(closeErr).Error("failed to close README file")
	}
	if err != nil {
		return fmt.Errorf("failed to execute readme template: %w", err)
	}
	return nil
}

func writeGitAttributes(root *os.Root) error {
	gitAttributesFile, err := root.Create(".gitattributes")
	if err != nil {
		return fmt.Errorf("failed to create git attributes file: %w", err)
	}

	defer func() {
		err = gitAttributesFile.Close()
		if err != nil {
			log.WithFields(log.Fields{
				common.SecurityField:    common.SecurityMedium,
				common.SecurityCWEField: common.SecurityCWEMissingReleaseOfFileDescriptor,
			}).Errorf("error closing file %q: %v", gitAttributesFile.Name(), err)
		}
	}()

	_, err = gitAttributesFile.WriteString(gitAttributesContents)
	if err != nil {
		return fmt.Errorf("failed to write git attributes: %w", err)
	}

	return nil
}

// writeManifests writes the manifests to the manifest.yaml file, truncating the file if it exists and appending the
// manifests in the order they are provided.
func writeManifests(root *os.Root, dirPath string, manifests []*apiclient.HydratedManifestDetails) error {
	// If the file exists, truncate it.
	// No need to use SecureJoin here, as the path is already sanitized.
	manifestPath := filepath.Join(dirPath, ManifestYaml)

	file, err := root.OpenFile(manifestPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to open manifest file: %w", err)
	}
	defer func() {
		err := file.Close()
		if err != nil {
			log.WithError(err).Error("failed to close file")
		}
	}()

	enc := yaml.NewEncoder(file)
	defer func() {
		err := enc.Close()
		if err != nil {
			log.WithError(err).Error("failed to close yaml encoder")
		}
	}()
	enc.SetIndent(2)

	for _, m := range manifests {
		obj := &unstructured.Unstructured{}
		err = json.Unmarshal([]byte(m.ManifestJSON), obj)
		if err != nil {
			return fmt.Errorf("failed to unmarshal manifest: %w", err)
		}
		err = enc.Encode(&obj.Object)
		if err != nil {
			return fmt.Errorf("failed to encode manifest: %w", err)
		}
	}
	return nil
}

// deleteManifest attempts to remove the "manifest.yaml" file located in the specified directory path
// within the given os.Root abstraction. If the file removal fails, it returns a wrapped error with context.
// This helper is used to clean up files which are written to disk but are still un-staged and a similar file exists in index.
func deleteManifest(root *os.Root, dirPath string) error {
	manifestPath := filepath.Join(dirPath, ManifestYaml)
	if err := root.Remove(manifestPath); err != nil {
		return fmt.Errorf("failed to remove manifest: %w", err)
	}
	return nil
}

// hasManifestChanged checks whether the "manifest.yaml" file in the specified directory path
// has been modified in the working tree compared to the version tracked in the current git index.
// Returns true if the file has changes staged or unstaged; otherwise, returns false.
// Relies on the provided git.Client's HasFileChanged method for underlying diff detection.
func hasManifestChanged(dirPath string, gitClient git.Client) (bool, error) {
	manifestPath := filepath.Join(dirPath, ManifestYaml)
	return gitClient.HasFileChanged(manifestPath)
}

// IsHydrated checks whether the given commit (commitSha) has already been hydrated with the specified Dry SHA (drySha).
// It does this by retrieving the commit note in the NoteNamespace and examining the DrySHA value.
// Returns true if the stored DrySHA matches the provided drySha, false if not or if no note exists.
// Gracefully handles missing notes as a normal outcome (not an error), but returns an error on retrieval or parse failures.
func IsHydrated(gitClient git.Client, drySha, commitSha string) (bool, error) {
	note, err := gitClient.GetCommitNote(commitSha, NoteNamespace)
	if err != nil {
		// an empty note or note not found is a valid and acceptable outcome in this context
		unwrappedError := errors.Unwrap(err)
		if unwrappedError != nil && strings.Contains(unwrappedError.Error(), "no note found") {
			return false, nil
		}
		return false, err
	}
	var commitNote CommitNote
	err = json.Unmarshal([]byte(note), &commitNote)
	if err != nil {
		return false, fmt.Errorf("json unmarshal failed %w", err)
	}
	return commitNote.DrySHA == drySha, nil
}

// AddNote attaches a commit note containing the specified dry SHA (`drySha`) to the given commit (`commitSha`)
// in the configured note namespace. The note is marshaled as JSON and pushed to the remote repository using
// the provided gitClient. Returns an error if marshalling or note addition fails.
func AddNote(gitClient git.Client, drySha, commitSha string) error {
	note := CommitNote{DrySHA: drySha}
	jsonBytes, err := json.Marshal(note)
	if err != nil {
		return fmt.Errorf("failed to marshal commit note: %w", err)
	}
	err = gitClient.AddAndPushNote(commitSha, NoteNamespace, string(jsonBytes))
	if err != nil {
		return fmt.Errorf("failed to add commit note: %w", err)
	}
	return nil
}
