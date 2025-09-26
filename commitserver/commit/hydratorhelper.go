package commit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/google/go-cmp/cmp"
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
func WriteForPaths(root *os.Root, repoUrl, drySha, targetBranch string, dryCommitMetadata *appv1.RevisionMetadata, paths []*apiclient.PathDetails, gitClient git.Client) error { //nolint:revive //FIXME(var-naming)
	hydratorMetadata, err := hydrator.GetCommitMetadata(repoUrl, drySha, dryCommitMetadata)
	if err != nil {
		return fmt.Errorf("failed to retrieve hydrator metadata: %w", err)
	}

	// Write the top-level readme.
	err = writeMetadata(root, "", hydratorMetadata)
	if err != nil {
		return fmt.Errorf("failed to write top-level hydrator metadata: %w", err)
	}

	// Write .gitattributes
	err = writeGitAttributes(root)
	if err != nil {
		return fmt.Errorf("failed to write git attributes: %w", err)
	}
	var atleastOneNewManifestExists bool
	for _, p := range paths {
		hydratePath := p.Path
		if hydratePath == "." {
			hydratePath = ""
		}

		// Only create directory if path is not empty (root directory case)
		if hydratePath != "" {
			err = root.MkdirAll(hydratePath, 0o755)
			if err != nil {
				return fmt.Errorf("failed to create path: %w", err)
			}
		}

		// Write the manifests
		err := writeManifests(root, targetBranch, hydratePath, p.Manifests, gitClient)
		if err != nil {
			if strings.EqualFold(err.Error(), fmt.Sprintf(ExistingManifestErrorPrefix, hydratePath)) {
				continue
			}
			return fmt.Errorf("failed to write manifests: %w", err)
		}
		// If even one new manifest exists then commit needs to happen else skip commit
		// once set to true do not override
		if !atleastOneNewManifestExists {
			atleastOneNewManifestExists = true
		}

		// Write hydrator.metadata containing information about the hydration process.
		hydratorMetadata := hydrator.HydratorCommitMetadata{
			Commands: p.Commands,
			DrySHA:   drySha,
			RepoURL:  repoUrl,
		}
		err = writeMetadata(root, hydratePath, hydratorMetadata)
		if err != nil {
			return fmt.Errorf("failed to write hydrator metadata: %w", err)
		}

		// Write README
		err = writeReadme(root, hydratePath, hydratorMetadata)
		if err != nil {
			return fmt.Errorf("failed to write readme: %w", err)
		}
	}
	// if no manifest changes then skip commit
	if !atleastOneNewManifestExists {
		return errors.New(NothingToCommitErrorMessage)
	}
	return nil
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
func writeManifests(root *os.Root, branch, dirPath string, manifests []*apiclient.HydratedManifestDetails, gitClient git.Client) error {
	// If the file exists, truncate it.
	// No need to use SecureJoin here, as the path is already sanitized.
	manifestPath := filepath.Join(dirPath, "manifest.yaml")

	// build the current manifest
	manifestYAML, err := renderManifestsToYAML(manifests)
	if err != nil {
		return err
	}
	// get the most recently hydrated minifest from git
	existingManifest, err := gitClient.GetLatestManifest(branch, manifestPath)
	if err != nil {
		return err
	}

	manifestChanged := hasManifestChanged(manifestYAML, existingManifest)
	if !manifestChanged {
		return fmt.Errorf(ExistingManifestErrorPrefix, dirPath)
	}

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

	if _, err := file.Write(manifestYAML); err != nil {
		return fmt.Errorf("failed to write manifests to file: %w", err)
	}

	return nil
}

func hasManifestChanged(currentManifest, existingManifest []byte) bool {
	if len(existingManifest) == 0 {
		return true
	}
	var currentObj, existingObj any
	if err := yaml.Unmarshal(currentManifest, &currentObj); err != nil {
		fmt.Printf("Error unmarshaling current: %v\n", err)
		return true
	}
	if err := yaml.Unmarshal(existingManifest, &existingObj); err != nil {
		fmt.Printf("Error unmarshaling existing: %v\n", err)
		return true
	}

	// Compare using go-cmp
	return !cmp.Equal(currentObj, existingObj)
}

func renderManifestsToYAML(manifests []*apiclient.HydratedManifestDetails) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	defer func() {
		err := enc.Close()
		if err != nil {
			log.WithError(err).Error("failed to close yaml encoder")
		}
	}()

	for _, m := range manifests {
		obj := &unstructured.Unstructured{}
		err := json.Unmarshal([]byte(m.ManifestJSON), obj)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
		}
		if err := enc.Encode(&obj.Object); err != nil {
			return nil, fmt.Errorf("failed to encode manifest: %w", err)
		}
	}
	return buf.Bytes(), nil
}

func IsHydrated(gitClient git.Client, drySha string) (bool, error) {
	note, err := gitClient.GetCommitNote(drySha, NoteNamespace)
	if err != nil {
		// an empty note or note not found is a valid and acceptable outcome in this context
		unwrappedError := errors.Unwrap(err)
		if strings.Contains(unwrappedError.Error(), "no note found") {
			return false, nil
		}
		return false, err
	}
	if !strings.Contains(note, drySha) {
		return false, fmt.Errorf("invalid structure of note %s", note)
	}
	return true, nil
}
