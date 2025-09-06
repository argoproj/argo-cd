package services

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	repo_mocks "github.com/argoproj/argo-cd/v3/reposerver/apiclient/mocks"
	"github.com/argoproj/argo-cd/v3/util/db"
	"github.com/argoproj/argo-cd/v3/util/settings"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestGetDirectories(t *testing.T) {
	type fields struct {
		submoduleEnabled  bool
		getRepository     func(ctx context.Context, url, project string) (*v1alpha1.Repository, error)
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
			getRepository: func(_ context.Context, _, _ string) (*v1alpha1.Repository, error) {
				return nil, errors.New("unable to get repos")
			},
		}, args: args{}, want: nil, wantErr: assert.Error},
		{name: "ErrorGettingDirs", fields: fields{
			getRepository: func(_ context.Context, _, _ string) (*v1alpha1.Repository, error) {
				return &v1alpha1.Repository{}, nil
			},
			getGitDirectories: func(_ context.Context, _ *apiclient.GitDirectoriesRequest) (*apiclient.GitDirectoriesResponse, error) {
				return nil, errors.New("unable to get dirs")
			},
		}, args: args{}, want: nil, wantErr: assert.Error},
		{name: "HappyCase", fields: fields{
			getRepository: func(_ context.Context, _, _ string) (*v1alpha1.Repository, error) {
				return &v1alpha1.Repository{
					Repo: "foo",
				}, nil
			},
			getGitDirectories: func(_ context.Context, _ *apiclient.GitDirectoriesRequest) (*apiclient.GitDirectoriesResponse, error) {
				return &apiclient.GitDirectoriesResponse{
					Paths: []string{"foo", "foo/bar", "bar/foo"},
				}, nil
			},
		}, args: args{
			repoURL: "foo",
		}, want: []string{"foo", "foo/bar", "bar/foo"}, wantErr: assert.NoError},
		{name: "ErrorVerifyingCommit", fields: fields{
			getRepository: func(_ context.Context, _, _ string) (*v1alpha1.Repository, error) {
				return &v1alpha1.Repository{}, nil
			},
			getGitDirectories: func(_ context.Context, _ *apiclient.GitDirectoriesRequest) (*apiclient.GitDirectoriesResponse, error) {
				return nil, errors.New("revision HEAD is not signed")
			},
		}, args: args{}, want: nil, wantErr: assert.Error},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &argoCDService{
				getRepository:                   tt.fields.getRepository,
				submoduleEnabled:                tt.fields.submoduleEnabled,
				getGitDirectoriesFromRepoServer: tt.fields.getGitDirectories,
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
		submoduleEnabled bool
		getRepository    func(ctx context.Context, url, project string) (*v1alpha1.Repository, error)
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
			getRepository: func(_ context.Context, _, _ string) (*v1alpha1.Repository, error) {
				return nil, errors.New("unable to get repos")
			},
		}, args: args{}, want: nil, wantErr: assert.Error},
		{name: "ErrorGettingFiles", fields: fields{
			getRepository: func(_ context.Context, _, _ string) (*v1alpha1.Repository, error) {
				return &v1alpha1.Repository{}, nil
			},
			getGitFiles: func(_ context.Context, _ *apiclient.GitFilesRequest) (*apiclient.GitFilesResponse, error) {
				return nil, errors.New("unable to get files")
			},
		}, args: args{}, want: nil, wantErr: assert.Error},
		{name: "HappyCase", fields: fields{
			getRepository: func(_ context.Context, _, _ string) (*v1alpha1.Repository, error) {
				return &v1alpha1.Repository{
					Repo: "foo",
				}, nil
			},
			getGitFiles: func(_ context.Context, _ *apiclient.GitFilesRequest) (*apiclient.GitFilesResponse, error) {
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
		{name: "ErrorVerifyingCommit", fields: fields{
			getRepository: func(_ context.Context, _, _ string) (*v1alpha1.Repository, error) {
				return &v1alpha1.Repository{}, nil
			},
			getGitFiles: func(_ context.Context, _ *apiclient.GitFilesRequest) (*apiclient.GitFilesResponse, error) {
				return nil, errors.New("revision HEAD is not signed")
			},
		}, args: args{}, want: nil, wantErr: assert.Error},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &argoCDService{
				getRepository:             tt.fields.getRepository,
				submoduleEnabled:          tt.fields.submoduleEnabled,
				getGitFilesFromRepoServer: tt.fields.getGitFiles,
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
	testNamespace := "test"
	clientset := fake.NewClientset()
	testDB := db.NewDB(testNamespace, settings.NewSettingsManager(t.Context(), clientset, testNamespace), clientset)
	service := NewArgoCDService(testDB, false, &repo_mocks.Clientset{}, false)
	assert.NotNil(t, service)
}
