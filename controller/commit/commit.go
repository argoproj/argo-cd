package commit

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"os"
	"os/exec"
	"path"
	"sigs.k8s.io/yaml"
	"text/template"
	"time"
)

/**
The commit package provides a way for the controller to push manifests to git.
*/

type Service interface {
	Commit(ManifestsRequest) (ManifestsResponse, error)
}

type ManifestsRequest struct {
	RepoURL           string
	TargetBranch      string
	DrySHA            string
	CommitAuthorName  string
	CommitAuthorEmail string
	CommitMessage     string
	CommitTime        time.Time
	Paths             []PathDetails
	Commands          []string
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

func NewService() Service {
	return &service{}
}

type service struct {
}

func (s *service) Commit(r ManifestsRequest) (ManifestsResponse, error) {
	logCtx := log.WithFields(log.Fields{"repo": r.RepoURL, "branch": r.TargetBranch, "drySHA": r.DrySHA})
	logCtx.Info("committing")

	// Create a temp dir with a UUID
	dirName, err := uuid.NewRandom()
	dirPath := path.Join("/tmp/_commit-service", dirName.String())
	err = os.MkdirAll(dirPath, os.ModePerm)
	if err != nil {
		return ManifestsResponse{}, fmt.Errorf("failed to create temp dir: %w", err)
	}

	// Clone the repo into the temp dir using the git CLI
	err = exec.Command("git", "clone", r.RepoURL, dirPath).Run()
	if err != nil {
		return ManifestsResponse{}, fmt.Errorf("failed to clone repo: %w", err)
	}

	// Checkout the target branch
	checkoutCmd := exec.Command("git", "checkout", r.TargetBranch)
	checkoutCmd.Dir = dirPath
	out, err := checkoutCmd.CombinedOutput()
	if err != nil {
		log.WithError(err).WithField("output", string(out)).Error("failed to checkout branch")
		return ManifestsResponse{}, fmt.Errorf("failed to checkout branch: %w", err)
	}

	// Write the manifests to the temp dir
	for _, p := range r.Paths {
		hydratePath := p.Path
		if hydratePath == "." {
			hydratePath = ""
		}
		err = os.MkdirAll(path.Join(dirPath, hydratePath), os.ModePerm)
		if err != nil {
			return ManifestsResponse{}, fmt.Errorf("failed to create path: %w", err)
		}

		// If the file exists, truncate it.
		err = os.Truncate(path.Join(dirPath, hydratePath, "manifest.yaml"), 0)
		if err != nil {
			return ManifestsResponse{}, fmt.Errorf("failed to empty manifest file: %w", err)
		}

		file, err := os.OpenFile(path.Join(dirPath, hydratePath, "manifest.yaml"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.ModePerm)
		if err != nil {
			return ManifestsResponse{}, fmt.Errorf("failed to open manifest file: %w", err)
		}
		defer func() {
			err := file.Close()
			if err != nil {
				log.WithError(err).Error("failed to close file")
			}
		}()
		for _, m := range p.Manifests {
			// Marshal the manifests
			mYaml, err := yaml.Marshal(m.Manifest.Object)
			if err != nil {
				return ManifestsResponse{}, fmt.Errorf("failed to marshal manifest: %w", err)
			}
			mYaml = append(mYaml, []byte("\n---\n\n")...)
			// Write the yaml to manifest.yaml
			_, err = file.Write(mYaml)
			if err != nil {
				return ManifestsResponse{}, fmt.Errorf("failed to write manifest: %w", err)
			}
		}
	}

	// Write hydrator.metadata containing information about the hydration process.
	hydratorMetadata := hydratorMetadataFile{
		Commands: r.Commands,
		DrySHA:   r.DrySHA,
		RepoURL:  r.RepoURL,
	}
	hydratorMetadataJson, err := json.MarshalIndent(hydratorMetadata, "", "  ")
	if err != nil {
		return ManifestsResponse{}, fmt.Errorf("failed to marshal hydrator metadata: %w", err)
	}
	err = os.WriteFile(path.Join(dirPath, "hydrator.metadata"), hydratorMetadataJson, os.ModePerm)
	if err != nil {
		return ManifestsResponse{}, fmt.Errorf("failed to write hydrator metadata: %w", err)
	}

	// Write README
	readmeTemplate := template.New("readme")
	readmeTemplate, err = readmeTemplate.Parse(manifestHydrationReadmeTemplate)
	if err != nil {
		return ManifestsResponse{}, fmt.Errorf("failed to parse readme template: %w", err)
	}
	// Create writer to template into
	readmeFile, err := os.Create(path.Join(dirPath, "README.md"))
	if err != nil && !os.IsExist(err) {
		return ManifestsResponse{}, fmt.Errorf("failed to create README file: %w", err)
	}
	err = readmeTemplate.Execute(readmeFile, hydratorMetadata)
	closeErr := readmeFile.Close()
	if closeErr != nil {
		logCtx.WithError(closeErr).Error("failed to close README file")
	}
	if err != nil {
		return ManifestsResponse{}, fmt.Errorf("failed to execute readme template: %w", err)
	}

	// Commit the changes
	addCmd := exec.Command("git", "add", ".")
	addCmd.Dir = dirPath
	out, err = addCmd.CombinedOutput()
	if err != nil {
		log.WithError(err).WithField("output", string(out)).Error("failed to add files")
		return ManifestsResponse{}, fmt.Errorf("failed to add files: %w", err)
	}

	// Set author name
	authorCmd := exec.Command("git", "config", "user.name", r.CommitAuthorName)
	authorCmd.Dir = dirPath
	out, err = authorCmd.CombinedOutput()
	if err != nil {
		log.WithError(err).WithField("output", string(out)).Error("failed to set author name")
		return ManifestsResponse{}, fmt.Errorf("failed to set author name: %w", err)
	}

	// Set author email
	emailCmd := exec.Command("git", "config", "user.email", r.CommitAuthorEmail)
	emailCmd.Dir = dirPath
	out, err = emailCmd.CombinedOutput()
	if err != nil {
		log.WithError(err).WithField("output", string(out)).Error("failed to set author email")
		return ManifestsResponse{}, fmt.Errorf("failed to set author email: %w", err)
	}

	commitCmd := exec.Command("git", "commit", "-m", r.CommitMessage)
	commitCmd.Dir = dirPath
	out, err = commitCmd.CombinedOutput()
	if err != nil {
		log.WithError(err).WithField("output", string(out)).Error("failed to commit files")
		return ManifestsResponse{}, fmt.Errorf("failed to commit: %w", err)
	}

	pushCmd := exec.Command("git", "push", "origin", r.TargetBranch)
	pushCmd.Dir = dirPath
	out, err = pushCmd.CombinedOutput()
	if err != nil {
		log.WithError(err).WithField("output", string(out)).Error("failed to push manifests")
		return ManifestsResponse{}, fmt.Errorf("failed to push: %w", err)
	}
	log.WithField("output", string(out)).Debug("pushed manifests to git")

	return ManifestsResponse{}, nil
}

type hydratorMetadataFile struct {
	Commands []string `json:"commands"`
	RepoURL  string   `json:"repoURL"`
	DrySHA   string   `json:"drySha"`
}

var manifestHydrationReadmeTemplate = `
# Manifest Hydration

To hydrate the manifests in this repository, run the following commands:

` + "```shell\n" + `
git clone {{ .RepoURL }}
# cd into the cloned directory
git checkout {{ .DrySHA }}
{{ range $command := .Commands -}}
{{ $command }}
{{ end -}}` + "```"
