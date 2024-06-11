package commit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/argoproj/argo-cd/v2/applicationset/services/github_app_auth"
	"github.com/argoproj/argo-cd/v2/commitserver/apiclient"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"os"
	"os/exec"
	"path"
	"strings"
	"text/template"
)

type Service struct {
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Commit(ctx context.Context, r *apiclient.ManifestsRequest) (*apiclient.ManifestsResponse, error) {
	var authorName, authorEmail, basicAuth string

	logCtx := log.WithFields(log.Fields{"repo": r.RepoUrl, "branch": r.TargetBranch, "drySHA": r.DrySha})

	if isGitHubApp(r.Repo) {
		info := github_app_auth.Authentication{
			Id:             r.Repo.GithubAppId,
			InstallationId: r.Repo.GithubAppInstallationId,
			PrivateKey:     r.Repo.GithubAppPrivateKey,
		}
		appInstall, err := getAppInstallation(info)
		if err != nil {
			return &apiclient.ManifestsResponse{}, err
		}
		token, err := appInstall.Token(ctx)
		if err != nil {
			return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to get access token: %w", err)
		}
		client, err := getGitHubAppClient(info)
		if err != nil {
			return &apiclient.ManifestsResponse{}, fmt.Errorf("cannot create github client: %w", err)
		}
		app, _, err := client.Apps.Get(ctx, "")
		if err != nil {
			return &apiclient.ManifestsResponse{}, fmt.Errorf("cannot get app info: %w", err)
		}
		appLogin := fmt.Sprintf("%s[bot]", app.GetSlug())
		user, _, err := getGitHubInstallationClient(appInstall).Users.Get(ctx, appLogin)
		if err != nil {
			return &apiclient.ManifestsResponse{}, fmt.Errorf("cannot get app user info: %w", err)
		}
		authorName = user.GetLogin()
		authorEmail = fmt.Sprintf("%d+%s@users.noreply.github.com", user.GetID(), user.GetLogin())
		basicAuth = fmt.Sprintf("x-access-token:%s", token)
	} else {
		logCtx.Warn("No github app credentials were found")
	}

	logCtx.Debug("Creating temp dir")
	dirName, err := uuid.NewRandom()
	if err != nil {
		return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to generate a uuid to create temp dir: %w", err)
	}
	// Don't need SecureJoin here, both parts are safe.
	dirPath := path.Join("/tmp/_commit-service", dirName.String())
	err = os.MkdirAll(dirPath, os.ModePerm)
	if err != nil {
		return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() {
		err := os.RemoveAll(dirPath)
		if err != nil {
			logCtx.WithError(err).Errorf("failed to remove temp dir %s", dirPath)
		}
	}()

	// Clone the repo into the temp dir using the git CLI
	logCtx.Debugf("Cloning repo %s", r.RepoUrl)
	authRepoUrl := r.RepoUrl
	if basicAuth != "" && strings.HasPrefix(authRepoUrl, "https://github.com/") {
		authRepoUrl = fmt.Sprintf("https://%s@github.com/%s", basicAuth, strings.TrimPrefix(authRepoUrl, "https://github.com/"))
	}
	cloneCmd := exec.Command("git", "clone", authRepoUrl, dirPath)
	out, err := cloneCmd.CombinedOutput()
	if err != nil {
		logCtx.WithError(err).WithField("output", string(out)).Error("failed to clone repo")
		return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to clone repo: %w", err)
	}

	if basicAuth != "" {
		// This is the dumbest kind of auth and should never make it in main branch
		// git config url."https://${TOKEN}@github.com/".insteadOf "https://github.com/"
		logCtx.Debugf("Setting auth")
		authCmd := exec.Command("git", "config", fmt.Sprintf("url.\"https://%s@github.com/\".insteadOf", basicAuth), "https://github.com/")
		authCmd.Dir = dirPath
		out, err = authCmd.CombinedOutput()
		if err != nil {
			logCtx.WithError(err).WithField("output", string(out)).Error("failed to set auth")
			return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to set auth: %w", err)
		}
	}

	if authorName != "" {
		// Set author name
		logCtx.Debugf("Setting author name %s", authorName)
		authorCmd := exec.Command("git", "config", "user.name", authorName)
		authorCmd.Dir = dirPath
		out, err = authorCmd.CombinedOutput()
		if err != nil {
			logCtx.WithError(err).WithField("output", string(out)).Error("failed to set author name")
			return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to set author name: %w", err)
		}
	}

	if authorEmail != "" {
		// Set author email
		logCtx.Debugf("Setting author email %s", authorEmail)
		emailCmd := exec.Command("git", "config", "user.email", authorEmail)
		emailCmd.Dir = dirPath
		out, err = emailCmd.CombinedOutput()
		if err != nil {
			logCtx.WithError(err).WithField("output", string(out)).Error("failed to set author email")
			return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to set author email: %w", err)
		}
	}

	// Checkout the sync branch
	logCtx.Debugf("Checking out sync branch %s", r.SyncBranch)
	checkoutCmd := exec.Command("git", "checkout", r.SyncBranch)
	checkoutCmd.Dir = dirPath
	out, err = checkoutCmd.CombinedOutput()
	if err != nil {
		// If the sync branch doesn't exist, create it as an orphan branch.
		if strings.Contains(string(out), "did not match any file(s) known to git") {
			logCtx.Debug("Sync branch does not exist, creating orphan branch")
			checkoutCmd = exec.Command("git", "switch", "--orphan", r.SyncBranch)
			checkoutCmd.Dir = dirPath
			out, err = checkoutCmd.CombinedOutput()
			if err != nil {
				logCtx.WithError(err).WithField("output", string(out)).Error("failed to create orphan branch")
				return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to create orphan branch: %w", err)
			}
		} else {
			logCtx.WithError(err).WithField("output", string(out)).Error("failed to checkout sync branch")
			return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to checkout sync branch: %w", err)
		}

		// Make an empty initial commit.
		logCtx.Debug("Making initial commit")
		commitCmd := exec.Command("git", "commit", "--allow-empty", "-m", "Initial commit")
		commitCmd.Dir = dirPath
		out, err = commitCmd.CombinedOutput()
		if err != nil {
			logCtx.WithError(err).WithField("output", string(out)).Error("failed to commit initial commit")
			return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to commit initial commit: %w", err)
		}

		// Push the commit.
		logCtx.Debug("Pushing initial commit")
		pushCmd := exec.Command("git", "push", "origin", r.SyncBranch)
		pushCmd.Dir = dirPath
		out, err = pushCmd.CombinedOutput()
		if err != nil {
			logCtx.WithError(err).WithField("output", string(out)).Error("failed to push sync branch")
			return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to push sync branch: %w", err)
		}
	}

	// Checkout the target branch
	logCtx.Debugf("Checking out target branch %s", r.TargetBranch)
	checkoutCmd = exec.Command("git", "checkout", r.TargetBranch)
	checkoutCmd.Dir = dirPath
	out, err = checkoutCmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "did not match any file(s) known to git") {
			// If the branch does not exist, create any empty branch based on the sync branch
			// First, checkout the sync branch.
			logCtx.Debug("Checking out sync branch")
			checkoutCmd = exec.Command("git", "checkout", r.SyncBranch)
			checkoutCmd.Dir = dirPath
			out, err = checkoutCmd.CombinedOutput()
			if err != nil {
				logCtx.WithError(err).WithField("output", string(out)).Error("failed to checkout sync branch")
				return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to checkout sync branch: %w", err)
			}

			logCtx.Debugf("Creating branch %s", r.TargetBranch)
			checkoutCmd = exec.Command("git", "checkout", "-b", r.TargetBranch)
			checkoutCmd.Dir = dirPath
			out, err = checkoutCmd.CombinedOutput()
			if err != nil {
				logCtx.WithError(err).WithField("output", string(out)).Error("failed to create branch")
				return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to create branch: %w", err)
			}
		} else {
			logCtx.WithError(err).WithField("output", string(out)).Error("failed to checkout branch")
			return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to checkout branch: %w", err)
		}
	}

	// Clear the repo contents using git rm
	logCtx.Debug("Clearing repo contents")
	rmCmd := exec.Command("git", "rm", "-r", "--ignore-unmatch", ".")
	rmCmd.Dir = dirPath
	out, err = rmCmd.CombinedOutput()
	if err != nil {
		logCtx.WithError(err).WithField("output", string(out)).Error("failed to clear repo contents")
		return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to clear repo contents: %w", err)
	}

	// Write hydrator.metadata containing information about the hydration process. This top-level metadata file is used
	// for the promoter. An additional metadata file is placed in each hydration destination directory, if applicable.
	logCtx.Debug("Writing top-level hydrator metadata")
	hydratorMetadata := hydratorMetadataFile{
		DrySHA:  r.DrySha,
		RepoURL: r.RepoUrl,
	}
	hydratorMetadataJson, err := json.MarshalIndent(hydratorMetadata, "", "  ")
	if err != nil {
		return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to marshal hydrator metadata: %w", err)
	}
	// No need to use SecureJoin here, as the path is already sanitized.
	hydratorMetadataPath := path.Join(dirPath, "hydrator.metadata")
	err = os.WriteFile(hydratorMetadataPath, hydratorMetadataJson, os.ModePerm)
	if err != nil {
		return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to write hydrator metadata: %w", err)
	}

	// Write the manifests to the temp dir
	for _, p := range r.Paths {
		hydratePath := p.Path
		if hydratePath == "." {
			hydratePath = ""
		}
		logCtx.Debugf("Writing manifests to %s", hydratePath)
		fullHydratePath, err := securejoin.SecureJoin(dirPath, hydratePath)
		if err != nil {
			return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to construct hydrate path: %w", err)
		}
		err = os.MkdirAll(fullHydratePath, os.ModePerm)
		if err != nil {
			return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to create path: %w", err)
		}

		// If the file exists, truncate it.
		// No need to use SecureJoin here, as the path is already sanitized.
		manifestPath := path.Join(fullHydratePath, "manifest.yaml")
		logCtx.Debugf("Emptying manifest file %s", manifestPath)
		if _, err := os.Stat(manifestPath); err == nil {
			err = os.Truncate(manifestPath, 0)
			if err != nil {
				return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to empty manifest file: %w", err)
			}
		}

		logCtx.Debugf("Opening manifest file %s", manifestPath)
		file, err := os.OpenFile(manifestPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.ModePerm)
		if err != nil {
			return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to open manifest file: %w", err)
		}
		defer func() {
			err := file.Close()
			if err != nil {
				logCtx.WithError(err).Error("failed to close file")
			}
		}()
		logCtx.Debugf("Writing manifests to %s", manifestPath)
		for _, m := range p.Manifests {
			obj := &unstructured.Unstructured{}
			err = json.Unmarshal([]byte(m.Manifest), obj)
			if err != nil {
				return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to unmarshal manifest: %w", err)
			}
			// Marshal the manifests
			buf := bytes.Buffer{}
			enc := yaml.NewEncoder(&buf)
			enc.SetIndent(2)
			err = enc.Encode(&obj)
			if err != nil {
				return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to encode manifest: %w", err)
			}
			mYaml := buf.Bytes()
			if err != nil {
				return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to marshal manifest: %w", err)
			}
			mYaml = append(mYaml, []byte("\n---\n\n")...)
			// Write the yaml to manifest.yaml
			_, err = file.Write(mYaml)
			if err != nil {
				return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to write manifest: %w", err)
			}
		}

		// Write hydrator.metadata containing information about the hydration process.
		logCtx.Debug("Writing hydrator metadata")
		hydratorMetadata := hydratorMetadataFile{
			Commands: p.Commands,
			DrySHA:   r.DrySha,
			RepoURL:  r.RepoUrl,
		}
		hydratorMetadataJson, err := json.MarshalIndent(hydratorMetadata, "", "  ")
		if err != nil {
			return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to marshal hydrator metadata: %w", err)
		}
		// No need to use SecureJoin here, as the path is already sanitized.
		hydratorMetadataPath := path.Join(fullHydratePath, "hydrator.metadata")
		err = os.WriteFile(hydratorMetadataPath, hydratorMetadataJson, os.ModePerm)
		if err != nil {
			return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to write hydrator metadata: %w", err)
		}

		// Write README
		logCtx.Debugf("Writing README")
		readmeTemplate := template.New("readme")
		readmeTemplate, err = readmeTemplate.Parse(manifestHydrationReadmeTemplate)
		if err != nil {
			return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to parse readme template: %w", err)
		}
		// Create writer to template into
		// No need to use SecureJoin here, as the path is already sanitized.
		readmePath := path.Join(fullHydratePath, "README.md")
		readmeFile, err := os.Create(readmePath)
		if err != nil && !os.IsExist(err) {
			return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to create README file: %w", err)
		}
		err = readmeTemplate.Execute(readmeFile, hydratorMetadata)
		closeErr := readmeFile.Close()
		if closeErr != nil {
			logCtx.WithError(closeErr).Error("failed to close README file")
		}
		if err != nil {
			return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to execute readme template: %w", err)
		}
	}

	// Commit the changes
	logCtx.Debugf("Committing changes")
	addCmd := exec.Command("git", "add", ".")
	addCmd.Dir = dirPath
	out, err = addCmd.CombinedOutput()
	if err != nil {
		logCtx.WithError(err).WithField("output", string(out)).Error("failed to add files")
		return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to add files: %w", err)
	}

	commitCmd := exec.Command("git", "commit", "-m", r.CommitMessage)
	commitCmd.Dir = dirPath
	out, err = commitCmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "nothing to commit, working tree clean") {
			logCtx.Info("no changes to commit")
			return &apiclient.ManifestsResponse{}, nil
		}
		logCtx.WithError(err).WithField("output", string(out)).Error("failed to commit files")
		return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to commit: %w", err)
	}

	logCtx.Debugf("Pushing changes")
	pushCmd := exec.Command("git", "push", "origin", r.TargetBranch)
	pushCmd.Dir = dirPath
	out, err = pushCmd.CombinedOutput()
	if err != nil {
		logCtx.WithError(err).WithField("output", string(out)).Error("failed to push manifests")
		return &apiclient.ManifestsResponse{}, fmt.Errorf("failed to push: %w", err)
	}
	logCtx.WithField("output", string(out)).Debug("pushed manifests to git")

	return &apiclient.ManifestsResponse{}, nil
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
