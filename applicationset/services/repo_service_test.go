package services

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	repo_mocks "github.com/argoproj/argo-cd/v2/reposerver/apiclient/mocks"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/settings"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
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
			getRepository: func(ctx context.Context, url, project string) (*v1alpha1.Repository, error) {
				return nil, errors.New("unable to get repos")
			},
		}, args: args{}, want: nil, wantErr: assert.Error},
		{name: "ErrorGettingDirs", fields: fields{
			getRepository: func(ctx context.Context, url, project string) (*v1alpha1.Repository, error) {
				return &v1alpha1.Repository{}, nil
			},
			getGitDirectories: func(ctx context.Context, req *apiclient.GitDirectoriesRequest) (*apiclient.GitDirectoriesResponse, error) {
				return nil, errors.New("unable to get dirs")
			},
		}, args: args{}, want: nil, wantErr: assert.Error},
		{name: "HappyCase", fields: fields{
			getRepository: func(ctx context.Context, url, project string) (*v1alpha1.Repository, error) {
				return &v1alpha1.Repository{
					Repo: "foo",
				}, nil
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
			getRepository: func(ctx context.Context, url, project string) (*v1alpha1.Repository, error) {
				return &v1alpha1.Repository{}, nil
			},
			getGitDirectories: func(ctx context.Context, req *apiclient.GitDirectoriesRequest) (*apiclient.GitDirectoriesResponse, error) {
				return nil, errors.New("revision HEAD is not signed")
			},
		}, args: args{}, want: nil, wantErr: assert.Error},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &argoCDService{
				getRepository:     tt.fields.getRepository,
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

func TestGetDirectoriesRepoFiltering(t *testing.T) {
	testNamespace := "test"
	type fields struct {
		submoduleEnabled           bool
		getRepositoryPreAssertions func(ctx context.Context, url, project string)
		getGitDirectories          func(ctx context.Context, req *apiclient.GitDirectoriesRequest) (*apiclient.GitDirectoriesResponse, error)
		repoSecrets                []*corev1.Secret
	}
	type args struct {
		ctx     context.Context
		repoURL string
		project string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []string
		wantErr assert.ErrorAssertionFunc
	}{
		{name: "NonExistentRepoResolution", fields: fields{
			getRepositoryPreAssertions: func(ctx context.Context, url, project string) {
				require.Equal(t, "", project)
			},
			getGitDirectories: func(ctx context.Context, req *apiclient.GitDirectoriesRequest) (*apiclient.GitDirectoriesResponse, error) {
				require.Equal(t, &v1alpha1.Repository{Repo: "does-not-exist"}, req.Repo)
				return &apiclient.GitDirectoriesResponse{
					Paths: []string{},
				}, nil
			},
			repoSecrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      "cred1",
						Annotations: map[string]string{
							common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
						},
						Labels: map[string]string{
							common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
						},
					},
					Data: map[string][]byte{
						"url":      []byte("git@github.com:argoproj/argo-cd.git"),
						"project":  []byte("some-proj"),
						"password": []byte("123456"),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      "cred2",
						Annotations: map[string]string{
							common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
						},
						Labels: map[string]string{
							common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
						},
					},
					Data: map[string][]byte{
						"url":      []byte("git@github.com:argoproj/argo-cd.git"),
						"password": []byte("123456"),
					},
				},
			},
		}, args: args{
			repoURL: "does-not-exist",
		}, want: []string{}, wantErr: assert.NoError},
		{name: "DefaultRepoResolution", fields: fields{
			getGitDirectories: func(ctx context.Context, req *apiclient.GitDirectoriesRequest) (*apiclient.GitDirectoriesResponse, error) {
				require.Equal(t, &v1alpha1.Repository{Repo: "git@github.com:argoproj/argo-cd.git", Password: "123456"}, req.Repo)
				return &apiclient.GitDirectoriesResponse{
					Paths: []string{},
				}, nil
			},
			getRepositoryPreAssertions: func(ctx context.Context, url, project string) {
				require.Equal(t, "", project)
			},
			repoSecrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      "cred1",
						Annotations: map[string]string{
							common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
						},
						Labels: map[string]string{
							common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
						},
					},
					Data: map[string][]byte{
						"url":      []byte("git@github.com:argoproj/argo-cd.git"),
						"project":  []byte("some-proj"),
						"password": []byte("123456"),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      "cred2",
						Annotations: map[string]string{
							common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
						},
						Labels: map[string]string{
							common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
						},
					},
					Data: map[string][]byte{
						"url":      []byte("git@github.com:argoproj/argo-cd.git"),
						"password": []byte("123456"),
					},
				},
			},
		}, args: args{
			repoURL: "git@github.com:argoproj/argo-cd.git",
		}, want: []string{}, wantErr: assert.NoError},
		{name: "TemplatedRepoResolution", fields: fields{
			getGitDirectories: func(ctx context.Context, req *apiclient.GitDirectoriesRequest) (*apiclient.GitDirectoriesResponse, error) {
				require.Equal(t, &v1alpha1.Repository{Repo: "git@github.com:argoproj/argo-cd.git", Password: "123456"}, req.Repo)
				return &apiclient.GitDirectoriesResponse{
					Paths: []string{},
				}, nil
			},
			getRepositoryPreAssertions: func(ctx context.Context, url, project string) {
				require.Equal(t, "", project)
			},
			repoSecrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      "cred1",
						Annotations: map[string]string{
							common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
						},
						Labels: map[string]string{
							common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
						},
					},
					Data: map[string][]byte{
						"url":      []byte("git@github.com:argoproj/argo-cd.git"),
						"project":  []byte("some-proj"),
						"password": []byte("123456"),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      "cred2",
						Annotations: map[string]string{
							common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
						},
						Labels: map[string]string{
							common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
						},
					},
					Data: map[string][]byte{
						"url":      []byte("git@github.com:argoproj/argo-cd.git"),
						"password": []byte("123456"),
					},
				},
			},
		}, args: args{
			repoURL: "git@github.com:argoproj/argo-cd.git",
			project: "{{ .some-proj }}",
		}, want: []string{}, wantErr: assert.NoError},
		{name: "ProjectRepoResolutionHappyPath", fields: fields{
			getGitDirectories: func(ctx context.Context, req *apiclient.GitDirectoriesRequest) (*apiclient.GitDirectoriesResponse, error) {
				require.Equal(t, &v1alpha1.Repository{Repo: "git@github.com:argoproj/argo-cd.git", Project: "some-proj", Password: "123456"}, req.Repo)
				return &apiclient.GitDirectoriesResponse{
					Paths: []string{},
				}, nil
			},
			getRepositoryPreAssertions: func(ctx context.Context, url, project string) {
				require.Equal(t, "some-proj", project)
			},
			repoSecrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      "cred1",
						Annotations: map[string]string{
							common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
						},
						Labels: map[string]string{
							common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
						},
					},
					Data: map[string][]byte{
						"url":      []byte("git@github.com:argoproj/argo-cd.git"),
						"project":  []byte("some-proj"),
						"password": []byte("123456"),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      "cred2",
						Annotations: map[string]string{
							common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
						},
						Labels: map[string]string{
							common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
						},
					},
					Data: map[string][]byte{
						"url":      []byte("git@github.com:argoproj/argo-cd.git"),
						"password": []byte("123456"),
					},
				},
			},
		}, args: args{
			project: "some-proj",
			repoURL: "git@github.com:argoproj/argo-cd.git",
		}, want: []string{}, wantErr: assert.NoError},
		{name: "NonExistingProjectRepoResolution", fields: fields{
			getGitDirectories: func(ctx context.Context, req *apiclient.GitDirectoriesRequest) (*apiclient.GitDirectoriesResponse, error) {
				require.Equal(t, &v1alpha1.Repository{Repo: "git@github.com:argoproj/argo-cd.git"}, req.Repo)
				return &apiclient.GitDirectoriesResponse{
					Paths: []string{},
				}, nil
			},
			getRepositoryPreAssertions: func(ctx context.Context, url, project string) {
				require.Equal(t, "does-not-exist-proj", project)
			},
			repoSecrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      "cred1",
						Annotations: map[string]string{
							common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
						},
						Labels: map[string]string{
							common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
						},
					},
					Data: map[string][]byte{
						"url":      []byte("git@github.com:argoproj/argo-cd.git"),
						"project":  []byte("some-proj"),
						"password": []byte("123456"),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      "cred2",
						Annotations: map[string]string{
							common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
						},
						Labels: map[string]string{
							common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
						},
					},
					Data: map[string][]byte{
						"url":      []byte("git@github.com:argoproj/argo-cd.git"),
						"project":  []byte("some-other-proj"),
						"password": []byte("123456"),
					},
				},
			},
		}, args: args{
			project: "does-not-exist-proj",
			repoURL: "git@github.com:argoproj/argo-cd.git",
		}, want: []string{}, wantErr: assert.NoError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-cm",
					Namespace: testNamespace,
					Labels: map[string]string{
						"app.kubernetes.io/part-of": "argocd",
					},
				},
				Data: nil,
			}
			var objs []runtime.Object
			for _, secret := range tt.fields.repoSecrets {
				objs = append(objs, secret)
			}

			clientset := fake.NewClientset(append(objs, &cm)...)
			testDB := db.NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)
			a := &argoCDService{
				getRepository: func(ctx context.Context, url, project string) (*v1alpha1.Repository, error) {
					if tt.fields.getRepositoryPreAssertions != nil {
						tt.fields.getRepositoryPreAssertions(ctx, url, project)
					}
					return testDB.GetRepository(ctx, url, project)
				},
				submoduleEnabled:  tt.fields.submoduleEnabled,
				getGitDirectories: tt.fields.getGitDirectories,
			}
			got, err := a.GetDirectories(tt.args.ctx, tt.args.repoURL, "", tt.args.project, false, false)
			if !tt.wantErr(t, err, fmt.Sprintf("GetFiles(%v, %v, %v)", tt.args.ctx, tt.args.repoURL, tt.args.project)) {
				return
			}
			assert.Equalf(t, tt.want, got, "GetFiles(%v, %v, %v)", tt.args.ctx, tt.args.repoURL, tt.args.project)
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
			getRepository: func(ctx context.Context, url, project string) (*v1alpha1.Repository, error) {
				return nil, errors.New("unable to get repos")
			},
		}, args: args{}, want: nil, wantErr: assert.Error},
		{name: "ErrorGettingFiles", fields: fields{
			getRepository: func(ctx context.Context, url, project string) (*v1alpha1.Repository, error) {
				return &v1alpha1.Repository{}, nil
			},
			getGitFiles: func(ctx context.Context, req *apiclient.GitFilesRequest) (*apiclient.GitFilesResponse, error) {
				return nil, errors.New("unable to get files")
			},
		}, args: args{}, want: nil, wantErr: assert.Error},
		{name: "HappyCase", fields: fields{
			getRepository: func(ctx context.Context, url, project string) (*v1alpha1.Repository, error) {
				return &v1alpha1.Repository{
					Repo: "foo",
				}, nil
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
		{name: "ErrorVerifyingCommit", fields: fields{
			getRepository: func(ctx context.Context, url, project string) (*v1alpha1.Repository, error) {
				return &v1alpha1.Repository{}, nil
			},
			getGitFiles: func(ctx context.Context, req *apiclient.GitFilesRequest) (*apiclient.GitFilesResponse, error) {
				return nil, errors.New("revision HEAD is not signed")
			},
		}, args: args{}, want: nil, wantErr: assert.Error},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &argoCDService{
				getRepository:    tt.fields.getRepository,
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

func TestGetFilesRepoFiltering(t *testing.T) {
	testNamespace := "test"
	type fields struct {
		submoduleEnabled           bool
		getRepositoryPreAssertions func(ctx context.Context, url, project string)
		getGitFiles                func(ctx context.Context, req *apiclient.GitFilesRequest) (*apiclient.GitFilesResponse, error)
		repoSecrets                []*corev1.Secret
	}
	type args struct {
		ctx     context.Context
		repoURL string
		project string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[string][]byte
		wantErr assert.ErrorAssertionFunc
	}{
		{name: "NonExistentRepoResolution", fields: fields{
			getGitFiles: func(ctx context.Context, req *apiclient.GitFilesRequest) (*apiclient.GitFilesResponse, error) {
				require.Equal(t, &v1alpha1.Repository{Repo: "does-not-exist"}, req.Repo)
				return &apiclient.GitFilesResponse{
					Map: map[string][]byte{},
				}, nil
			},
			getRepositoryPreAssertions: func(ctx context.Context, url, project string) {
				require.Equal(t, "", project)
			},
			repoSecrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      "cred1",
						Annotations: map[string]string{
							common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
						},
						Labels: map[string]string{
							common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
						},
					},
					Data: map[string][]byte{
						"url":      []byte("git@github.com:argoproj/argo-cd.git"),
						"project":  []byte("some-proj"),
						"password": []byte("123456"),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      "cred2",
						Annotations: map[string]string{
							common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
						},
						Labels: map[string]string{
							common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
						},
					},
					Data: map[string][]byte{
						"url":      []byte("git@github.com:argoproj/argo-cd.git"),
						"password": []byte("123456"),
					},
				},
			},
		}, args: args{
			repoURL: "does-not-exist",
		}, want: map[string][]byte{}, wantErr: assert.NoError},
		{name: "DefaultRepoResolution", fields: fields{
			getGitFiles: func(ctx context.Context, req *apiclient.GitFilesRequest) (*apiclient.GitFilesResponse, error) {
				require.Equal(t, &v1alpha1.Repository{Repo: "git@github.com:argoproj/argo-cd.git", Password: "123456"}, req.Repo)
				return &apiclient.GitFilesResponse{
					Map: map[string][]byte{},
				}, nil
			},
			getRepositoryPreAssertions: func(ctx context.Context, url, project string) {
				require.Equal(t, "", project)
			},
			repoSecrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      "cred1",
						Annotations: map[string]string{
							common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
						},
						Labels: map[string]string{
							common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
						},
					},
					Data: map[string][]byte{
						"url":      []byte("git@github.com:argoproj/argo-cd.git"),
						"project":  []byte("some-proj"),
						"password": []byte("123456"),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      "cred2",
						Annotations: map[string]string{
							common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
						},
						Labels: map[string]string{
							common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
						},
					},
					Data: map[string][]byte{
						"url":      []byte("git@github.com:argoproj/argo-cd.git"),
						"password": []byte("123456"),
					},
				},
			},
		}, args: args{
			repoURL: "git@github.com:argoproj/argo-cd.git",
		}, want: map[string][]byte{}, wantErr: assert.NoError},
		{name: "TemplatedRepoResolution", fields: fields{
			getGitFiles: func(ctx context.Context, req *apiclient.GitFilesRequest) (*apiclient.GitFilesResponse, error) {
				require.Equal(t, &v1alpha1.Repository{Repo: "git@github.com:argoproj/argo-cd.git", Password: "123456"}, req.Repo)
				return &apiclient.GitFilesResponse{
					Map: map[string][]byte{},
				}, nil
			},
			getRepositoryPreAssertions: func(ctx context.Context, url, project string) {
				require.Equal(t, "", project)
			},
			repoSecrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      "cred1",
						Annotations: map[string]string{
							common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
						},
						Labels: map[string]string{
							common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
						},
					},
					Data: map[string][]byte{
						"url":      []byte("git@github.com:argoproj/argo-cd.git"),
						"project":  []byte("some-proj"),
						"password": []byte("123456"),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      "cred2",
						Annotations: map[string]string{
							common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
						},
						Labels: map[string]string{
							common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
						},
					},
					Data: map[string][]byte{
						"url":      []byte("git@github.com:argoproj/argo-cd.git"),
						"password": []byte("123456"),
					},
				},
			},
		}, args: args{
			repoURL: "git@github.com:argoproj/argo-cd.git",
			project: "{{ .some-proj }}",
		}, want: map[string][]byte{}, wantErr: assert.NoError},
		{name: "ProjectRepoResolutionHappyPath", fields: fields{
			getGitFiles: func(ctx context.Context, req *apiclient.GitFilesRequest) (*apiclient.GitFilesResponse, error) {
				require.Equal(t, &v1alpha1.Repository{Repo: "git@github.com:argoproj/argo-cd.git", Project: "some-proj", Password: "123456"}, req.Repo)
				return &apiclient.GitFilesResponse{
					Map: map[string][]byte{},
				}, nil
			},
			getRepositoryPreAssertions: func(ctx context.Context, url, project string) {
				require.Equal(t, "some-proj", project)
			},
			repoSecrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      "cred1",
						Annotations: map[string]string{
							common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
						},
						Labels: map[string]string{
							common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
						},
					},
					Data: map[string][]byte{
						"url":      []byte("git@github.com:argoproj/argo-cd.git"),
						"project":  []byte("some-proj"),
						"password": []byte("123456"),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      "cred2",
						Annotations: map[string]string{
							common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
						},
						Labels: map[string]string{
							common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
						},
					},
					Data: map[string][]byte{
						"url":      []byte("git@github.com:argoproj/argo-cd.git"),
						"password": []byte("123456"),
					},
				},
			},
		}, args: args{
			project: "some-proj",
			repoURL: "git@github.com:argoproj/argo-cd.git",
		}, want: map[string][]byte{}, wantErr: assert.NoError},
		{name: "NonExistingProjectRepoResolution", fields: fields{
			getGitFiles: func(ctx context.Context, req *apiclient.GitFilesRequest) (*apiclient.GitFilesResponse, error) {
				require.Equal(t, &v1alpha1.Repository{Repo: "git@github.com:argoproj/argo-cd.git"}, req.Repo)
				return &apiclient.GitFilesResponse{
					Map: map[string][]byte{},
				}, nil
			},
			getRepositoryPreAssertions: func(ctx context.Context, url, project string) {
				require.Equal(t, "does-not-exist-proj", project)
			},
			repoSecrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      "cred1",
						Annotations: map[string]string{
							common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
						},
						Labels: map[string]string{
							common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
						},
					},
					Data: map[string][]byte{
						"url":      []byte("git@github.com:argoproj/argo-cd.git"),
						"project":  []byte("some-proj"),
						"password": []byte("123456"),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      "cred2",
						Annotations: map[string]string{
							common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
						},
						Labels: map[string]string{
							common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
						},
					},
					Data: map[string][]byte{
						"url":      []byte("git@github.com:argoproj/argo-cd.git"),
						"project":  []byte("some-other-proj"),
						"password": []byte("123456"),
					},
				},
			},
		}, args: args{
			project: "does-not-exist-proj",
			repoURL: "git@github.com:argoproj/argo-cd.git",
		}, want: map[string][]byte{}, wantErr: assert.NoError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-cm",
					Namespace: testNamespace,
					Labels: map[string]string{
						"app.kubernetes.io/part-of": "argocd",
					},
				},
				Data: nil,
			}
			var objs []runtime.Object
			for _, secret := range tt.fields.repoSecrets {
				objs = append(objs, secret)
			}

			clientset := fake.NewClientset(append(objs, &cm)...)
			testDB := db.NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)
			a := &argoCDService{
				getRepository: func(ctx context.Context, url, project string) (*v1alpha1.Repository, error) {
					if tt.fields.getRepositoryPreAssertions != nil {
						tt.fields.getRepositoryPreAssertions(ctx, url, project)
					}
					return testDB.GetRepository(ctx, url, project)
				},
				submoduleEnabled: tt.fields.submoduleEnabled,
				getGitFiles:      tt.fields.getGitFiles,
			}
			got, err := a.GetFiles(tt.args.ctx, tt.args.repoURL, "", tt.args.project, "", false, false)
			if !tt.wantErr(t, err, fmt.Sprintf("GetFiles(%v, %v, %v)", tt.args.ctx, tt.args.repoURL, tt.args.project)) {
				return
			}
			assert.Equalf(t, tt.want, got, "GetFiles(%v, %v, %v)", tt.args.ctx, tt.args.repoURL, tt.args.project)
		})
	}
}

func TestNewArgoCDService(t *testing.T) {
	testNamespace := "test"
	clientset := fake.NewClientset()
	testDB := db.NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)
	service, err := NewArgoCDService(testDB, false, &repo_mocks.Clientset{}, false)
	require.NoError(t, err)
	assert.NotNil(t, service)
}
