package services

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	repo_mocks "github.com/argoproj/argo-cd/v2/reposerver/apiclient/mocks"
	"github.com/argoproj/argo-cd/v2/util/git"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func TestGetDirectories(t *testing.T) {
	type fields struct {
		storecreds        git.CredsStore
		submoduleEnabled  bool
		listRepositories  func(ctx context.Context) ([]*v1alpha1.Repository, error)
		getGitDirectories func(ctx context.Context, req *apiclient.GitDirectoriesRequest) (*apiclient.GitDirectoriesResponse, error)
	}
	type args struct {
		ctx             context.Context
		repoURL         string
		revision        string
		noRevisionCache bool
		verifyCommit    bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []string
		wantErr assert.ErrorAssertionFunc
	}{
		{name: "ErrorGettingRepos", fields: fields{
			listRepositories: func(ctx context.Context) ([]*v1alpha1.Repository, error) {
				return nil, fmt.Errorf("unable to get repos")
			},
		}, args: args{}, want: nil, wantErr: assert.Error},
		{name: "ErrorGettingDirs", fields: fields{
			listRepositories: func(ctx context.Context) ([]*v1alpha1.Repository, error) {
				return []*v1alpha1.Repository{{}}, nil
			},
			getGitDirectories: func(ctx context.Context, req *apiclient.GitDirectoriesRequest) (*apiclient.GitDirectoriesResponse, error) {
				return nil, fmt.Errorf("unable to get dirs")
			},
		}, args: args{}, want: nil, wantErr: assert.Error},
		{name: "HappyCase", fields: fields{
			listRepositories: func(ctx context.Context) ([]*v1alpha1.Repository, error) {
				return []*v1alpha1.Repository{{
					Repo: "foo",
				}}, nil
			},
			getGitDirectories: func(ctx context.Context, req *apiclient.GitDirectoriesRequest) (*apiclient.GitDirectoriesResponse, error) {
				return &apiclient.GitDirectoriesResponse{
					Paths: []string{"foo", "foo/bar", "bar/foo"},
				}, nil
			},
		}, args: args{
			repoURL: "foo",
		}, want: []string{"foo", "foo/bar", "bar/foo"}, wantErr: assert.NoError},
		{name: "ErrorVerifyingCommit", fields: fields{
			listRepositories: func(ctx context.Context) ([]*v1alpha1.Repository, error) {
				return []*v1alpha1.Repository{{}}, nil
			},
			getGitDirectories: func(ctx context.Context, req *apiclient.GitDirectoriesRequest) (*apiclient.GitDirectoriesResponse, error) {
				return nil, fmt.Errorf("revision HEAD is not signed")
			},
		}, args: args{}, want: nil, wantErr: assert.Error},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &argoCDService{
				listRepositories:  tt.fields.listRepositories,
				storecreds:        tt.fields.storecreds,
				submoduleEnabled:  tt.fields.submoduleEnabled,
				getGitDirectories: tt.fields.getGitDirectories,
			}
			got, err := a.GetDirectories(tt.args.ctx, tt.args.repoURL, tt.args.revision, "", tt.args.noRevisionCache, tt.args.verifyCommit)
			if !tt.wantErr(t, err, fmt.Sprintf("GetDirectories(%v, %v, %v, %v)", tt.args.ctx, tt.args.repoURL, tt.args.revision, tt.args.noRevisionCache)) {
				return
			}
			assert.Equalf(t, tt.want, got, "GetDirectories(%v, %v, %v, %v)", tt.args.ctx, tt.args.repoURL, tt.args.revision, tt.args.noRevisionCache)
		})
	}
}

func TestGetFiles(t *testing.T) {
	type fields struct {
		storecreds       git.CredsStore
		submoduleEnabled bool
		listRepositories func(ctx context.Context) ([]*v1alpha1.Repository, error)
		getGitFiles      func(ctx context.Context, req *apiclient.GitFilesRequest) (*apiclient.GitFilesResponse, error)
	}
	type args struct {
		ctx             context.Context
		repoURL         string
		revision        string
		pattern         string
		noRevisionCache bool
		verifyCommit    bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[string][]byte
		wantErr assert.ErrorAssertionFunc
	}{
		{name: "ErrorGettingRepos", fields: fields{
			listRepositories: func(ctx context.Context) ([]*v1alpha1.Repository, error) {
				return nil, fmt.Errorf("unable to get repos")
			},
		}, args: args{}, want: nil, wantErr: assert.Error},
		{name: "ErrorGettingFiles", fields: fields{
			listRepositories: func(ctx context.Context) ([]*v1alpha1.Repository, error) {
				return []*v1alpha1.Repository{{}}, nil
			},
			getGitFiles: func(ctx context.Context, req *apiclient.GitFilesRequest) (*apiclient.GitFilesResponse, error) {
				return nil, fmt.Errorf("unable to get files")
			},
		}, args: args{}, want: nil, wantErr: assert.Error},
		{name: "HappyCase", fields: fields{
			listRepositories: func(ctx context.Context) ([]*v1alpha1.Repository, error) {
				return []*v1alpha1.Repository{{
					Repo: "foo",
				}}, nil
			},
			getGitFiles: func(ctx context.Context, req *apiclient.GitFilesRequest) (*apiclient.GitFilesResponse, error) {
				return &apiclient.GitFilesResponse{
					Map: map[string][]byte{
						"foo.json": []byte("hello: world!"),
						"bar.yaml": []byte("yay: appsets"),
					},
				}, nil
			},
		}, args: args{
			repoURL: "foo",
		}, want: map[string][]byte{
			"foo.json": []byte("hello: world!"),
			"bar.yaml": []byte("yay: appsets"),
		}, wantErr: assert.NoError},
		{name: "NoRepoFoundFallback", fields: fields{
			listRepositories: func(ctx context.Context) ([]*v1alpha1.Repository, error) {
				return []*v1alpha1.Repository{}, nil
			},
			getGitFiles: func(ctx context.Context, req *apiclient.GitFilesRequest) (*apiclient.GitFilesResponse, error) {
				require.Equal(t, &v1alpha1.Repository{Repo: "foo"}, req.Repo)
				return &apiclient.GitFilesResponse{
					Map: map[string][]byte{},
				}, nil
			},
		}, args: args{
			repoURL: "foo",
		}, want: map[string][]byte{}, wantErr: assert.NoError},
		{name: "RepoWithProjectFoundFallback", fields: fields{
			listRepositories: func(ctx context.Context) ([]*v1alpha1.Repository, error) {
				return []*v1alpha1.Repository{{Repo: "foo", Project: "default"}}, nil
			},
			getGitFiles: func(ctx context.Context, req *apiclient.GitFilesRequest) (*apiclient.GitFilesResponse, error) {
				require.Equal(t, &v1alpha1.Repository{Repo: "foo", Project: "default"}, req.Repo)
				return &apiclient.GitFilesResponse{
					Map: map[string][]byte{},
				}, nil
			},
		}, args: args{
			repoURL: "foo",
		}, want: map[string][]byte{}, wantErr: assert.NoError},
		{name: "MultipleReposWithEmptyProjectFoundFallback", fields: fields{
			listRepositories: func(ctx context.Context) ([]*v1alpha1.Repository, error) {
				return []*v1alpha1.Repository{{Repo: "foo", Project: "default"}, {Repo: "foo", Project: ""}}, nil
			},
			getGitFiles: func(ctx context.Context, req *apiclient.GitFilesRequest) (*apiclient.GitFilesResponse, error) {
				require.Equal(t, &v1alpha1.Repository{Repo: "foo", Project: ""}, req.Repo)
				return &apiclient.GitFilesResponse{
					Map: map[string][]byte{},
				}, nil
			},
		}, args: args{
			repoURL: "foo",
		}, want: map[string][]byte{}, wantErr: assert.NoError},
		{name: "MultipleReposFoundFallback", fields: fields{
			listRepositories: func(ctx context.Context) ([]*v1alpha1.Repository, error) {
				return []*v1alpha1.Repository{{Repo: "foo", Project: "default"}, {Repo: "foo", Project: "bar"}}, nil
			},
			getGitFiles: func(ctx context.Context, req *apiclient.GitFilesRequest) (*apiclient.GitFilesResponse, error) {
				require.Equal(t, &v1alpha1.Repository{Repo: "foo", Project: "default"}, req.Repo)
				return &apiclient.GitFilesResponse{
					Map: map[string][]byte{},
				}, nil
			},
		}, args: args{
			repoURL: "foo",
		}, want: map[string][]byte{}, wantErr: assert.NoError},
		{name: "ErrorVerifyingCommit", fields: fields{
			listRepositories: func(ctx context.Context) ([]*v1alpha1.Repository, error) {
				return []*v1alpha1.Repository{{}}, nil
			},
			getGitFiles: func(ctx context.Context, req *apiclient.GitFilesRequest) (*apiclient.GitFilesResponse, error) {
				return nil, fmt.Errorf("revision HEAD is not signed")
			},
		}, args: args{}, want: nil, wantErr: assert.Error},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &argoCDService{
				listRepositories: tt.fields.listRepositories,
				storecreds:       tt.fields.storecreds,
				submoduleEnabled: tt.fields.submoduleEnabled,
				getGitFiles:      tt.fields.getGitFiles,
			}
			got, err := a.GetFiles(tt.args.ctx, tt.args.repoURL, tt.args.revision, tt.args.pattern, "", tt.args.noRevisionCache, tt.args.verifyCommit)
			if !tt.wantErr(t, err, fmt.Sprintf("GetFiles(%v, %v, %v, %v, %v)", tt.args.ctx, tt.args.repoURL, tt.args.revision, tt.args.pattern, tt.args.noRevisionCache)) {
				return
			}
			assert.Equalf(t, tt.want, got, "GetFiles(%v, %v, %v, %v, %v)", tt.args.ctx, tt.args.repoURL, tt.args.revision, tt.args.pattern, tt.args.noRevisionCache)
		})
	}
}

func TestNewArgoCDService(t *testing.T) {
	service, err := NewArgoCDService(func(ctx context.Context) ([]*v1alpha1.Repository, error) {
		return []*v1alpha1.Repository{{}}, nil
	}, false, &repo_mocks.Clientset{}, false)
	require.NoError(t, err)
	assert.NotNil(t, service)
}
