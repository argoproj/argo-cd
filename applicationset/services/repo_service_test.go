package services

import (
	"context"
	"fmt"
	"testing"

	"github.com/argoproj/argo-cd/v2/applicationset/services/mocks"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	repo_mocks "github.com/argoproj/argo-cd/v2/reposerver/apiclient/mocks"
	db_mocks "github.com/argoproj/argo-cd/v2/util/db/mocks"
	"github.com/argoproj/argo-cd/v2/util/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func TestGetDirectories(t *testing.T) {

	type fields struct {
		repositoriesDBFuncs   []func(*mocks.RepositoryDB)
		storecreds            git.CredsStore
		submoduleEnabled      bool
		repoServerClientFuncs []func(*repo_mocks.RepoServerServiceClient)
	}
	type args struct {
		ctx             context.Context
		repoURL         string
		revision        string
		noRevisionCache bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []string
		wantErr assert.ErrorAssertionFunc
	}{
		{name: "ErrorGettingRepos", fields: fields{
			repositoriesDBFuncs: []func(*mocks.RepositoryDB){
				func(db *mocks.RepositoryDB) {
					db.On("GetRepository", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("unable to get repos"))
				},
			},
		}, args: args{}, want: nil, wantErr: assert.Error},
		{name: "ErrorGettingDirs", fields: fields{
			repositoriesDBFuncs: []func(*mocks.RepositoryDB){
				func(db *mocks.RepositoryDB) {
					db.On("GetRepository", mock.Anything, mock.Anything).Return(&v1alpha1.Repository{}, nil)
				},
			},
			repoServerClientFuncs: []func(*repo_mocks.RepoServerServiceClient){
				func(client *repo_mocks.RepoServerServiceClient) {
					client.On("GetGitDirectories", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("unable to get dirs"))
				},
			},
		}, args: args{}, want: nil, wantErr: assert.Error},
		{name: "HappyCase", fields: fields{
			repositoriesDBFuncs: []func(*mocks.RepositoryDB){
				func(db *mocks.RepositoryDB) {
					db.On("GetRepository", mock.Anything, mock.Anything).Return(&v1alpha1.Repository{}, nil)
				},
			},
			repoServerClientFuncs: []func(*repo_mocks.RepoServerServiceClient){
				func(client *repo_mocks.RepoServerServiceClient) {
					client.On("GetGitDirectories", mock.Anything, mock.Anything).Return(&apiclient.GitDirectoriesResponse{
						Paths: []string{"foo", "foo/bar", "bar/foo"},
					}, nil)
				},
			},
		}, args: args{}, want: []string{"foo", "foo/bar", "bar/foo"}, wantErr: assert.NoError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDb := &mocks.RepositoryDB{}
			mockRepoClient := &repo_mocks.RepoServerServiceClient{}
			// decorate the mocks
			for i := range tt.fields.repositoriesDBFuncs {
				tt.fields.repositoriesDBFuncs[i](mockDb)
			}
			for i := range tt.fields.repoServerClientFuncs {
				tt.fields.repoServerClientFuncs[i](mockRepoClient)
			}

			a := &argoCDService{
				repositoriesDB:      mockDb,
				storecreds:          tt.fields.storecreds,
				submoduleEnabled:    tt.fields.submoduleEnabled,
				repoServerClientSet: &repo_mocks.Clientset{RepoServerServiceClient: mockRepoClient},
			}
			got, err := a.GetDirectories(tt.args.ctx, tt.args.repoURL, tt.args.revision, tt.args.noRevisionCache)
			if !tt.wantErr(t, err, fmt.Sprintf("GetDirectories(%v, %v, %v, %v)", tt.args.ctx, tt.args.repoURL, tt.args.revision, tt.args.noRevisionCache)) {
				return
			}
			assert.Equalf(t, tt.want, got, "GetDirectories(%v, %v, %v, %v)", tt.args.ctx, tt.args.repoURL, tt.args.revision, tt.args.noRevisionCache)
		})
	}
}

func TestGetFiles(t *testing.T) {
	type fields struct {
		repositoriesDBFuncs   []func(*mocks.RepositoryDB)
		storecreds            git.CredsStore
		submoduleEnabled      bool
		repoServerClientFuncs []func(*repo_mocks.RepoServerServiceClient)
	}
	type args struct {
		ctx             context.Context
		repoURL         string
		revision        string
		pattern         string
		noRevisionCache bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[string][]byte
		wantErr assert.ErrorAssertionFunc
	}{
		{name: "ErrorGettingRepos", fields: fields{
			repositoriesDBFuncs: []func(*mocks.RepositoryDB){
				func(db *mocks.RepositoryDB) {
					db.On("GetRepository", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("unable to get repos"))
				},
			},
		}, args: args{}, want: nil, wantErr: assert.Error},
		{name: "ErrorGettingFiles", fields: fields{
			repositoriesDBFuncs: []func(*mocks.RepositoryDB){
				func(db *mocks.RepositoryDB) {
					db.On("GetRepository", mock.Anything, mock.Anything).Return(&v1alpha1.Repository{}, nil)
				},
			},
			repoServerClientFuncs: []func(*repo_mocks.RepoServerServiceClient){
				func(client *repo_mocks.RepoServerServiceClient) {
					client.On("GetGitFiles", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("unable to get files"))
				},
			},
		}, args: args{}, want: nil, wantErr: assert.Error},
		{name: "HappyCase", fields: fields{
			repositoriesDBFuncs: []func(*mocks.RepositoryDB){
				func(db *mocks.RepositoryDB) {
					db.On("GetRepository", mock.Anything, mock.Anything).Return(&v1alpha1.Repository{}, nil)
				},
			},
			repoServerClientFuncs: []func(*repo_mocks.RepoServerServiceClient){
				func(client *repo_mocks.RepoServerServiceClient) {
					client.On("GetGitFiles", mock.Anything, mock.Anything).Return(&apiclient.GitFilesResponse{
						Map: map[string][]byte{
							"foo.json": []byte("hello: world!"),
							"bar.yaml": []byte("yay: appsets"),
						},
					}, nil)
				},
			},
		}, args: args{}, want: map[string][]byte{
			"foo.json": []byte("hello: world!"),
			"bar.yaml": []byte("yay: appsets"),
		}, wantErr: assert.NoError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDb := &mocks.RepositoryDB{}
			mockRepoClient := &repo_mocks.RepoServerServiceClient{}
			// decorate the mocks
			for i := range tt.fields.repositoriesDBFuncs {
				tt.fields.repositoriesDBFuncs[i](mockDb)
			}
			for i := range tt.fields.repoServerClientFuncs {
				tt.fields.repoServerClientFuncs[i](mockRepoClient)
			}

			a := &argoCDService{
				repositoriesDB:      mockDb,
				storecreds:          tt.fields.storecreds,
				submoduleEnabled:    tt.fields.submoduleEnabled,
				repoServerClientSet: &repo_mocks.Clientset{RepoServerServiceClient: mockRepoClient},
			}
			got, err := a.GetFiles(tt.args.ctx, tt.args.repoURL, tt.args.revision, tt.args.pattern, tt.args.noRevisionCache)
			if !tt.wantErr(t, err, fmt.Sprintf("GetFiles(%v, %v, %v, %v, %v)", tt.args.ctx, tt.args.repoURL, tt.args.revision, tt.args.pattern, tt.args.noRevisionCache)) {
				return
			}
			assert.Equalf(t, tt.want, got, "GetFiles(%v, %v, %v, %v, %v)", tt.args.ctx, tt.args.repoURL, tt.args.revision, tt.args.pattern, tt.args.noRevisionCache)
		})
	}
}

func TestNewArgoCDService(t *testing.T) {
	service, err := NewArgoCDService(&db_mocks.ArgoDB{}, false, &repo_mocks.Clientset{}, false)
	assert.NoError(t, err, err)
	assert.NotNil(t, service)
}
