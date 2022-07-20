package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/git"
)

// RepositoryDB Is a lean facade for ArgoDB,
// Using a lean interface makes it more easy to test the functionality the git generator uses
type RepositoryDB interface {
	GetRepository(ctx context.Context, url string) (*v1alpha1.Repository, error)
}

type argoCDService struct {
	repositoriesDB RepositoryDB
	storecreds     git.CredsStore
}

type Repos interface {

	// GetFiles returns content of files (not directories) within the target repo
	GetFiles(ctx context.Context, repoURL string, revision string, pattern string) (map[string][]byte, error)

	// GetDirectories returns a list of directories (not files) within the target repo
	GetDirectories(ctx context.Context, repoURL string, revision string) ([]string, error)
}

func NewArgoCDService(db db.ArgoDB, gitCredStore git.CredsStore, repoServerAddress string) Repos {

	return &argoCDService{
		repositoriesDB: db.(RepositoryDB),
		storecreds:     gitCredStore,
	}
}

func (a *argoCDService) GetFiles(ctx context.Context, repoURL string, revision string, pattern string) (map[string][]byte, error) {
	repo, err := a.repositoriesDB.GetRepository(ctx, repoURL)
	if err != nil {
		return nil, fmt.Errorf("Error in GetRepository: %w", err)
	}

	gitRepoClient, err := git.NewClient(repo.Repo, repo.GetGitCreds(a.storecreds), repo.IsInsecure(), repo.IsLFSEnabled(), repo.Proxy)

	if err != nil {
		return nil, err
	}

	err = checkoutRepo(gitRepoClient, revision)
	if err != nil {
		return nil, err
	}

	paths, err := gitRepoClient.LsFiles(pattern)
	if err != nil {
		return nil, fmt.Errorf("Error during listing files of local repo: %w", err)
	}

	res := map[string][]byte{}
	for _, filePath := range paths {
		bytes, err := os.ReadFile(filepath.Join(gitRepoClient.Root(), filePath))
		if err != nil {
			return nil, err
		}
		res[filePath] = bytes
	}

	return res, nil
}

func (a *argoCDService) GetDirectories(ctx context.Context, repoURL string, revision string) ([]string, error) {

	repo, err := a.repositoriesDB.GetRepository(ctx, repoURL)
	if err != nil {
		return nil, fmt.Errorf("Error in GetRepository: %w", err)
	}

	gitRepoClient, err := git.NewClient(repo.Repo, repo.GetGitCreds(a.storecreds), repo.IsInsecure(), repo.IsLFSEnabled(), repo.Proxy)
	if err != nil {
		return nil, err
	}

	err = checkoutRepo(gitRepoClient, revision)
	if err != nil {
		return nil, err
	}

	filteredPaths := []string{}

	repoRoot := gitRepoClient.Root()

	if err := filepath.Walk(repoRoot, func(path string, info os.FileInfo, fnErr error) error {
		if fnErr != nil {
			return fnErr
		}
		if !info.IsDir() { // Skip files: directories only
			return nil
		}

		fname := info.Name()
		if strings.HasPrefix(fname, ".") { // Skip all folders starts with "."
			return filepath.SkipDir
		}

		relativePath, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}

		if relativePath == "." { // Exclude '.' from results
			return nil
		}

		filteredPaths = append(filteredPaths, relativePath)

		return nil
	}); err != nil {
		return nil, err
	}

	return filteredPaths, nil

}

func checkoutRepo(gitRepoClient git.Client, revision string) error {
	err := gitRepoClient.Init()
	if err != nil {
		return fmt.Errorf("Error during initializing repo: %w", err)
	}

	err = gitRepoClient.Fetch(revision)
	if err != nil {
		return fmt.Errorf("Error during fetching repo: %w", err)
	}

	commitSHA, err := gitRepoClient.LsRemote(revision)
	if err != nil {
		return fmt.Errorf("Error during fetching commitSHA: %w", err)
	}
	err = gitRepoClient.Checkout(commitSHA, true)
	if err != nil {
		return fmt.Errorf("Error during repo checkout: %w", err)
	}
	return nil
}
