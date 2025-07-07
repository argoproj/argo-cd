package commit

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/commitserver/apiclient"
	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/git"
	"github.com/argoproj/argo-cd/v3/util/io"
)

var sprigFuncMap = sprig.GenericFuncMap() // a singleton for better performance

func init() {
	// Avoid allowing the user to learn things about the environment.
	delete(sprigFuncMap, "env")
	delete(sprigFuncMap, "expandenv")
	delete(sprigFuncMap, "getHostByName")
}

// WriteForPaths writes the manifests, hydrator.metadata, and README.md files for each path in the provided paths. It
// also writes a root-level hydrator.metadata file containing the repo URL and dry SHA.
func WriteForPaths(root *os.Root, repoUrl, drySha string, dryCommitMetadata *appv1.RevisionMetadata, paths []*apiclient.PathDetails) error { //nolint:revive //FIXME(var-naming)
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

	// Write the top-level readme.
	err := writeMetadata(root, "", hydratorMetadataFile{DrySHA: drySha, RepoURL: repoUrl, Author: author, Subject: subject, Body: bodyMinusTrailers, Date: date, References: references})
	if err != nil {
		return fmt.Errorf("failed to write top-level hydrator metadata: %w", err)
	}

	for _, p := range paths {
		hydratePath := p.Path
		if hydratePath == "." {
			hydratePath = ""
		}

		err = mkdirAll(root, hydratePath)
		if err != nil {
			return fmt.Errorf("failed to create path: %w", err)
		}

		// Write the manifests
		err = writeManifests(root, hydratePath, p.Manifests)
		if err != nil {
			return fmt.Errorf("failed to write manifests: %w", err)
		}

		// Write hydrator.metadata containing information about the hydration process.
		hydratorMetadata := hydratorMetadataFile{
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
	return nil
}

// writeMetadata writes the metadata to the hydrator.metadata file.
func writeMetadata(root *os.Root, dirPath string, metadata hydratorMetadataFile) error {
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
func writeReadme(root *os.Root, dirPath string, metadata hydratorMetadataFile) error {
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

// writeManifests writes the manifests to the manifest.yaml file, truncating the file if it exists and appending the
// manifests in the order they are provided.
func writeManifests(root *os.Root, dirPath string, manifests []*apiclient.HydratedManifestDetails) error {
	// If the file exists, truncate it.
	// No need to use SecureJoin here, as the path is already sanitized.
	manifestPath := filepath.Join(dirPath, "manifest.yaml")

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

// mkdirAll creates the directory and all its parents if they do not exist. It returns an error if the directory
// cannot be.
func mkdirAll(root *os.Root, dirPath string) error {
	parts := strings.Split(dirPath, string(os.PathSeparator))
	builtPath := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		builtPath = filepath.Join(builtPath, part)
		err := root.Mkdir(builtPath, os.ModePerm)
		if err != nil {
			if errors.Is(err, fs.ErrExist) {
				log.WithError(err).Warnf("path %s already exists, skipping", dirPath)
				continue
			}
			return fmt.Errorf("failed to create path: %w", err)
		}
	}
	return nil
}
