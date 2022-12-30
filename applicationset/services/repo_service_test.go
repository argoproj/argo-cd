package services

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"

	"github.com/argoproj/argo-cd/v2/applicationset/utils"
	"github.com/argoproj/argo-cd/v2/util/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

const argocdExampleAppCommitID = "08f72e2a309beab929d9fd14626071b1a61a47f9"

type ArgocdRepositoryMock struct {
	mock *mock.Mock
}

func (a ArgocdRepositoryMock) GetRepository(ctx context.Context, url string) (*v1alpha1.Repository, error) {
	args := a.mock.Called(ctx, url)

	return args.Get(0).(*v1alpha1.Repository), args.Error(1)

}

func TestGetDirectories(t *testing.T) {

	// Hardcode a specific revision to changes to argocd-example-apps from regressing this test:
	//   Author: Alexander Matyushentsev <Alexander_Matyushentsev@intuit.com>
	//   Date:   Sun Jan 31 09:54:53 2021 -0800
	//   chore: downgrade kustomize guestbook image tag (#73)
	exampleRepoRevision := argocdExampleAppCommitID

	for _, c := range []struct {
		name          string
		repoURL       string
		revision      string
		repoRes       *v1alpha1.Repository
		repoErr       error
		expected      []string
		expectedError error
	}{
		{
			name:     "All child folders should be returned",
			repoURL:  "https://github.com/argoproj/argocd-example-apps/",
			revision: exampleRepoRevision,
			repoRes: &v1alpha1.Repository{
				Repo: "https://github.com/argoproj/argocd-example-apps/",
			},
			repoErr: nil,
			expected: []string{"apps", "apps/templates", "blue-green", "blue-green/templates", "guestbook", "helm-dependency",
				"helm-guestbook", "helm-guestbook/templates", "helm-hooks", "jsonnet-guestbook", "jsonnet-guestbook-tla",
				"ksonnet-guestbook", "ksonnet-guestbook/components", "ksonnet-guestbook/environments", "ksonnet-guestbook/environments/default",
				"ksonnet-guestbook/environments/dev", "ksonnet-guestbook/environments/prod", "kustomize-guestbook", "plugins", "plugins/kasane",
				"plugins/kustomized-helm", "plugins/kustomized-helm/overlays", "pre-post-sync", "sock-shop", "sock-shop/base", "sync-waves"},
		},
		{
			name:     "If GetRepository returns an error, it should pass back to caller",
			repoURL:  "https://github.com/argoproj/argocd-example-apps/",
			revision: exampleRepoRevision,
			repoRes: &v1alpha1.Repository{
				Repo: "https://github.com/argoproj/argocd-example-apps/",
			},
			repoErr:       fmt.Errorf("Simulated error from GetRepository"),
			expected:      nil,
			expectedError: fmt.Errorf("Error in GetRepository: Simulated error from GetRepository"),
		},
		{
			name: "Test against repository containing no directories",
			// Here I picked an arbitrary repository in argoproj-labs, with a commit containing no folders.
			repoURL:  "https://github.com/argoproj-labs/argo-workflows-operator/",
			revision: "5f50933a576833b73b7a172909d8545a108685f4",
			repoRes: &v1alpha1.Repository{
				Repo: "https://github.com/argoproj-labs/argo-workflows-operator/",
			},
			repoErr:  nil,
			expected: []string{},
		},
	} {
		cc := c
		t.Run(cc.name, func(t *testing.T) {
			argocdRepositoryMock := ArgocdRepositoryMock{mock: &mock.Mock{}}

			argocdRepositoryMock.mock.On("GetRepository", mock.Anything, cc.repoURL).Return(cc.repoRes, cc.repoErr)

			argocd := argoCDService{
				repositoriesDB: argocdRepositoryMock,
				keyLock:        utils.NewKeyLock(),
			}

			got, err := argocd.GetDirectories(context.TODO(), cc.repoURL, cc.revision)

			if cc.expectedError != nil {
				assert.EqualError(t, err, cc.expectedError.Error())
			} else {
				sort.Strings(got)
				sort.Strings(cc.expected)

				assert.Equal(t, got, cc.expected)
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetFiles(t *testing.T) {

	// Hardcode a specific commit, so that changes to argoproj/argocd-example-apps/ don't break our tests
	// "chore: downgrade kustomize guestbook image tag (#73)"
	commitID := argocdExampleAppCommitID

	tests := []struct {
		name     string
		repoURL  string
		revision string
		pattern  string
		repoRes  *v1alpha1.Repository
		repoErr  error

		expectSubsetOfPaths []string
		doesNotContainPaths []string
		expectedError       error
	}{
		{
			name: "pull a specific revision of example apps and verify the list is expected",
			repoRes: &v1alpha1.Repository{
				Insecure:              true,
				InsecureIgnoreHostKey: true,
				Repo:                  "https://github.com/argoproj/argocd-example-apps/",
			},
			repoURL:  "https://github.com/argoproj/argocd-example-apps/",
			revision: commitID,
			pattern:  "*",
			expectSubsetOfPaths: []string{
				"apps/Chart.yaml",
				"apps/templates/helm-guestbook.yaml",
				"apps/templates/helm-hooks.yaml",
				"apps/templates/kustomize-guestbook.yaml",
				"apps/templates/namespaces.yaml",
				"apps/templates/sync-waves.yaml",
				"apps/values.yaml",
				"blue-green/.helmignore",
				"blue-green/Chart.yaml",
				"blue-green/README.md",
				"blue-green/templates/NOTES.txt",
				"blue-green/templates/rollout.yaml",
				"blue-green/templates/services.yaml",
				"blue-green/values.yaml",
				"guestbook/guestbook-ui-deployment.yaml",
				"guestbook/guestbook-ui-svc.yaml",
				"kustomize-guestbook/guestbook-ui-deployment.yaml",
				"kustomize-guestbook/guestbook-ui-svc.yaml",
				"kustomize-guestbook/kustomization.yaml",
			},
		},
		{
			name: "pull an invalid revision, and confirm an error is returned",
			repoRes: &v1alpha1.Repository{
				Insecure:              true,
				InsecureIgnoreHostKey: true,
				Repo:                  "https://github.com/argoproj/argocd-example-apps/",
			},
			repoURL:             "https://github.com/argoproj/argocd-example-apps/",
			revision:            "this-tag-does-not-exist",
			pattern:             "*",
			expectSubsetOfPaths: []string{},
			expectedError:       fmt.Errorf("Error during fetching repo: `git fetch origin this-tag-does-not-exist --tags --force --prune` failed exit status 128: fatal: couldn't find remote ref this-tag-does-not-exist"),
		},
		{
			name: "pull a specific revision of example apps, and use a ** pattern",
			repoRes: &v1alpha1.Repository{
				Insecure:              true,
				InsecureIgnoreHostKey: true,
				Repo:                  "https://github.com/argoproj/argocd-example-apps/",
			},
			repoURL:  "https://github.com/argoproj/argocd-example-apps/",
			revision: commitID,
			pattern:  "**/*.yaml",
			expectSubsetOfPaths: []string{
				"apps/Chart.yaml",
				"apps/templates/helm-guestbook.yaml",
				"apps/templates/helm-hooks.yaml",
				"apps/templates/kustomize-guestbook.yaml",
				"apps/templates/namespaces.yaml",
				"apps/templates/sync-waves.yaml",
				"apps/values.yaml",
				"blue-green/templates/rollout.yaml",
				"blue-green/templates/services.yaml",
				"blue-green/values.yaml",
				"guestbook/guestbook-ui-deployment.yaml",
				"guestbook/guestbook-ui-svc.yaml",
				"kustomize-guestbook/guestbook-ui-deployment.yaml",
				"kustomize-guestbook/guestbook-ui-svc.yaml",
				"kustomize-guestbook/kustomization.yaml",
			},
			doesNotContainPaths: []string{
				"blue-green/.helmignore",
				"blue-green/README.md",
				"blue-green/templates/NOTES.txt",
			},
		},
	}

	for _, cc := range tests {

		// Get all the paths for a repository, and confirm that the expected subset of paths is found (or the expected error is returned)
		t.Run(cc.name, func(t *testing.T) {
			argocdRepositoryMock := ArgocdRepositoryMock{mock: &mock.Mock{}}

			argocdRepositoryMock.mock.On("GetRepository", mock.Anything, cc.repoURL).Return(cc.repoRes, cc.repoErr)

			argocd := argoCDService{
				repositoriesDB: argocdRepositoryMock,
			}

			getPathsRes, err := argocd.GetFiles(context.Background(), cc.repoURL, cc.revision, cc.pattern)

			if cc.expectedError == nil {

				assert.NoError(t, err)
				for _, path := range cc.expectSubsetOfPaths {
					assert.Contains(t, getPathsRes, path, "Unable to locate path: %s", path)
				}

				for _, shouldNotContain := range cc.doesNotContainPaths {
					assert.NotContains(t, getPathsRes, shouldNotContain, "GetPaths should not contain %s", shouldNotContain)
				}

			} else {
				assert.EqualError(t, err, cc.expectedError.Error())
			}
		})
	}
}

func TestConcurrentGitRequestsDirectories(t *testing.T) {
	repoMock := ArgocdRepositoryMock{mock: &mock.Mock{}}
	argocdExampleURL := "https://github.com/argoproj/argocd-example-apps/"
	commitID := argocdExampleAppCommitID
	// https://github.com/argoproj/argocd-example-apps/tree/67de934fd7f22062a4e2ac8b8d20cfc97f2b4e7f/guestbook
	lessDirectoriesCommit := "67de934fd7f22062a4e2ac8b8d20cfc97f2b4e7f"

	repoMock.mock.On("GetRepository", mock.Anything, argocdExampleURL).Return(&v1alpha1.Repository{
		Insecure:              true,
		InsecureIgnoreHostKey: true,
		Repo:                  "https://github.com/argoproj/argocd-example-apps/",
	}, nil)

	type fields struct {
		repositoriesDB   RepositoryDB
		storecreds       git.CredsStore
		submoduleEnabled bool
		keyLock          *utils.KeyLock
		workers          int
	}
	type args struct {
		ctx      context.Context
		repoURL  string
		revision []string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[string][]string
		wantErr assert.ErrorAssertionFunc
	}{
		{name: "Single", fields: fields{
			repositoriesDB: repoMock,
			keyLock:        utils.NewKeyLock(),
			workers:        1,
		}, args: args{
			ctx:      context.Background(),
			repoURL:  argocdExampleURL,
			revision: []string{commitID},
		}, want: map[string][]string{commitID: {"apps", "apps/templates", "blue-green", "blue-green/templates", "guestbook", "helm-dependency",
			"helm-guestbook", "helm-guestbook/templates", "helm-hooks", "jsonnet-guestbook", "jsonnet-guestbook-tla",
			"ksonnet-guestbook", "ksonnet-guestbook/components", "ksonnet-guestbook/environments", "ksonnet-guestbook/environments/default",
			"ksonnet-guestbook/environments/dev", "ksonnet-guestbook/environments/prod", "kustomize-guestbook", "plugins", "plugins/kasane",
			"plugins/kustomized-helm", "plugins/kustomized-helm/overlays", "pre-post-sync", "sock-shop", "sock-shop/base", "sync-waves"}}, wantErr: assert.NoError},
		{name: "Many", fields: fields{
			repositoriesDB: repoMock,
			keyLock:        utils.NewKeyLock(),
			workers:        5,
		}, args: args{
			ctx:      context.Background(),
			repoURL:  argocdExampleURL,
			revision: []string{lessDirectoriesCommit, commitID, commitID, lessDirectoriesCommit, commitID},
		}, want: map[string][]string{
			commitID: {"apps", "apps/templates", "blue-green", "blue-green/templates", "guestbook", "helm-dependency",
				"helm-guestbook", "helm-guestbook/templates", "helm-hooks", "jsonnet-guestbook", "jsonnet-guestbook-tla",
				"ksonnet-guestbook", "ksonnet-guestbook/components", "ksonnet-guestbook/environments", "ksonnet-guestbook/environments/default",
				"ksonnet-guestbook/environments/dev", "ksonnet-guestbook/environments/prod", "kustomize-guestbook", "plugins", "plugins/kasane",
				"plugins/kustomized-helm", "plugins/kustomized-helm/overlays", "pre-post-sync", "sock-shop", "sock-shop/base", "sync-waves"},
			lessDirectoriesCommit: {"guestbook", "guestbook/components", "guestbook/environments", "guestbook/environments/default"}},
			wantErr: assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &argoCDService{
				repositoriesDB:   tt.fields.repositoriesDB,
				storecreds:       tt.fields.storecreds,
				submoduleEnabled: tt.fields.submoduleEnabled,
				keyLock:          tt.fields.keyLock,
			}
			var wg sync.WaitGroup
			for i := 0; i < len(tt.args.revision); i++ {
				wg.Add(1)
				revision := tt.args.revision[i]
				go func() {
					got, err := a.GetDirectories(tt.args.ctx, tt.args.repoURL, revision)
					if !tt.wantErr(t, err, fmt.Sprintf("GetDirectories(%v, %v, %v)", tt.args.ctx, tt.args.repoURL, revision)) {
						return
					}
					assert.Equalf(t, tt.want[revision], got, "GetDirectories(%v, %v, %v)", tt.args.ctx, tt.args.repoURL, revision)
					wg.Done()
				}()
			}
			wg.Wait()
		})
	}
}

func TestConcurrentGitRequestsFiles(t *testing.T) {
	type fields struct {
		repositoriesDB   RepositoryDB
		storecreds       git.CredsStore
		submoduleEnabled bool
		keyLock          *utils.KeyLock
	}
	type args struct {
		ctx      context.Context
		repoURL  string
		revision string
		pattern  string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[string][]byte
		wantErr assert.ErrorAssertionFunc
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &argoCDService{
				repositoriesDB:   tt.fields.repositoriesDB,
				storecreds:       tt.fields.storecreds,
				submoduleEnabled: tt.fields.submoduleEnabled,
				keyLock:          tt.fields.keyLock,
			}
			got, err := a.GetFiles(tt.args.ctx, tt.args.repoURL, tt.args.revision, tt.args.pattern)
			if !tt.wantErr(t, err, fmt.Sprintf("GetFiles(%v, %v, %v, %v)", tt.args.ctx, tt.args.repoURL, tt.args.revision, tt.args.pattern)) {
				return
			}
			assert.Equalf(t, tt.want, got, "GetFiles(%v, %v, %v, %v)", tt.args.ctx, tt.args.repoURL, tt.args.revision, tt.args.pattern)
		})
	}
}
