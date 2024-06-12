package commit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"text/template"

	"os"
	"path"
	"text/template"

	securejoin "github.com/cyphar/filepath-securejoin"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// HydratorHelper is an interface for writing metadata, readme, and manifests for hydrator.
type HydratorHelper interface {
	// WriteMetadata writes the metadata to the hydrator.metadata file.
	WriteMetadata(metadata hydratorMetadataFile, subDirPath string) error
	// WriteReadme writes the readme to the README.md file.
	WriteReadme(readme hydratorMetadataFile, subDirPath string) error
	// WriteManifests writes the manifests to the manifest.yaml file, truncating the file if it exists and appending the
	// manifests in the order they are provided.
	WriteManifests(manifests []*apiclient.ManifestDetails, subDirPath string) error
}

type hydratorHelper struct {
	dirPath string
}

// NewHydratorHelper creates a new HydratorHelper. The dirPath is the root directory where the hydrator files will be
// written, i.e. a git repository.
func newHydratorHelper(dirPath string) HydratorHelper {
	return &hydratorHelper{dirPath: dirPath}
}

// WriteMetadata writes the metadata to the hydrator.metadata file.
func (h *hydratorHelper) WriteMetadata(metadata hydratorMetadataFile, subDirPath string) error {
	fullPath, err := securejoin.SecureJoin(h.dirPath, subDirPath)
	if err != nil {
		return fmt.Errorf("failed to join path: %w", err)
	}

	hydratorMetadataJson, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal hydrator metadata: %w", err)
	}
	// No need to use SecureJoin here, as the path is already sanitized.
	hydratorMetadataPath := path.Join(fullPath, "hydrator.metadata")
	err = os.WriteFile(hydratorMetadataPath, hydratorMetadataJson, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to write hydrator metadata: %w", err)
	}
	return nil
}

// WriteReadme writes the readme to the README.md file.
func (h *hydratorHelper) WriteReadme(metadata hydratorMetadataFile, subDirPath string) error {
	fullPath, err := securejoin.SecureJoin(h.dirPath, subDirPath)
	if err != nil {
		return fmt.Errorf("failed to join path: %w", err)
	}

	readmeTemplate := template.New("readme")
	readmeTemplate, err = readmeTemplate.Parse(manifestHydrationReadmeTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse readme template: %w", err)
	}
	// Create writer to template into
	// No need to use SecureJoin here, as the path is already sanitized.
	readmePath := path.Join(fullPath, "README.md")
	readmeFile, err := os.Create(readmePath)
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

// WriteManifests writes the manifests to the manifest.yaml file, truncating the file if it exists and appending the
// manifests in the order they are provided.
func (h *hydratorHelper) WriteManifests(manifests []*apiclient.ManifestDetails, subDirPath string) error {
	fullHydratePath, err := securejoin.SecureJoin(h.dirPath, subDirPath)
	if err != nil {
		return fmt.Errorf("failed to join path: %w", err)
	}

	// If the file exists, truncate it.
	// No need to use SecureJoin here, as the path is already sanitized.
	manifestPath := path.Join(fullHydratePath, "manifest.yaml")
	if _, err := os.Stat(manifestPath); err == nil {
		err = os.Truncate(manifestPath, 0)
		if err != nil {
			return fmt.Errorf("failed to empty manifest file: %w", err)
		}
	}

	file, err := os.OpenFile(manifestPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to open manifest file: %w", err)
	}
	defer func() {
		err := file.Close()
		if err != nil {
			log.WithError(err).Error("failed to close file")
		}
	}()
	for _, m := range manifests {
		obj := &unstructured.Unstructured{}
		err = json.Unmarshal([]byte(m.Manifest), obj)
		if err != nil {
			return fmt.Errorf("failed to unmarshal manifest: %w", err)
		}
		// Marshal the manifests
		buf := bytes.Buffer{}
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		err = enc.Encode(&obj.Object)
		if err != nil {
			return fmt.Errorf("failed to encode manifest: %w", err)
		}
		mYaml := buf.Bytes()
		if err != nil {
			return fmt.Errorf("failed to marshal manifest: %w", err)
		}
		mYaml = append(mYaml, []byte("\n---\n\n")...)
		// Write the yaml to manifest.yaml
		_, err = file.Write(mYaml)
		if err != nil {
			return fmt.Errorf("failed to write manifest: %w", err)
		}
	}
	return nil
}
