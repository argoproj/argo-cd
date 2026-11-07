package repository

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/util/helm"
	helmmocks "github.com/argoproj/argo-cd/v3/util/helm/mocks"
)

func newHelmRepositoryTestService(t *testing.T) (*Service, *helmmocks.Client) {
	t.Helper()
	client := helmmocks.NewClient(t)
	return &Service{
		initConstants: RepoServerInitConstants{HelmRegistryMaxIndexSize: 1024},
		newHelmClient: func(_ string, _ helm.Creds, _ bool, _ string, _ string, _ ...helm.ClientOpts) helm.Client {
			return client
		},
	}, client
}

func TestTestRepositoryMissingRepo(t *testing.T) {
	for _, req := range []*apiclient.TestRepositoryRequest{nil, {}} {
		_, err := (&Service{}).TestRepository(t.Context(), req)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	}
}

func TestTestRepositoryCoalescesHelmChecks(t *testing.T) {
	for _, enableOCI := range []bool{false, true} {
		for _, checkErr := range []error{nil, errors.New("repository unavailable")} {
			synctest.Test(t, func(t *testing.T) {
				service, client := newHelmRepositoryTestService(t)
				release := make(chan struct{})
				var expectation *mock.Call
				if enableOCI {
					expectation = client.On("TestHelmOCI", mock.Anything).Return(true, checkErr)
				} else {
					expectation = client.On("GetIndex", mock.Anything, false, int64(1024)).Return(&helm.Index{}, checkErr)
				}
				expectation.Run(func(mock.Arguments) { <-release }).Once()
				req := &apiclient.TestRepositoryRequest{Repo: &v1alpha1.Repository{
					Repo: "charts.example.test", Type: "helm", EnableOCI: enableOCI,
				}}
				const callers = 10
				results := make(chan error, callers)
				for range callers {
					go func() {
						_, err := service.TestRepository(t.Context(), req)
						results <- err
					}()
				}
				synctest.Wait()
				close(release)
				for range callers {
					require.ErrorIs(t, <-results, checkErr)
				}

				// A completed result, including an error, must not be cached.
				if enableOCI {
					client.On("TestHelmOCI", mock.Anything).Return(true, nil).Once()
				} else {
					client.On("GetIndex", mock.Anything, false, int64(1024)).Return(&helm.Index{}, nil).Once()
				}
				_, err := service.TestRepository(t.Context(), req)
				require.NoError(t, err)
			})
		}
	}
}

func TestTestRepositoryHelmCancellationIsIndependent(t *testing.T) {
	for _, cancelFirst := range []bool{false, true} {
		synctest.Test(t, func(t *testing.T) {
			service, client := newHelmRepositoryTestService(t)
			release := make(chan struct{})
			var checkCtx context.Context
			client.On("GetIndex", mock.Anything, false, int64(1024)).Run(func(args mock.Arguments) {
				checkCtx = args.Get(0).(context.Context)
				<-release
			}).Return(&helm.Index{}, nil).Once()
			req := &apiclient.TestRepositoryRequest{Repo: &v1alpha1.Repository{Repo: "https://charts.example.test", Type: "helm"}}
			firstCtx, cancelFirstCall := context.WithCancel(t.Context())
			defer cancelFirstCall()
			secondCtx, cancelSecondCall := context.WithCancel(t.Context())
			defer cancelSecondCall()
			firstResult, secondResult := make(chan error, 1), make(chan error, 1)
			go func() {
				_, err := service.TestRepository(firstCtx, req)
				firstResult <- err
			}()
			synctest.Wait()
			go func() {
				_, err := service.TestRepository(secondCtx, req)
				secondResult <- err
			}()
			synctest.Wait()
			canceled, remaining := secondResult, firstResult
			if cancelFirst {
				cancelFirstCall()
				canceled, remaining = firstResult, secondResult
			} else {
				cancelSecondCall()
			}
			require.ErrorIs(t, <-canceled, context.Canceled)
			require.NoError(t, checkCtx.Err())
			close(release)
			require.NoError(t, <-remaining)
		})
	}
}

func TestTestRepositoryHelmCheckTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service, client := newHelmRepositoryTestService(t)
		client.On("GetIndex", mock.Anything, false, int64(1024)).Run(func(args mock.Arguments) {
			<-args.Get(0).(context.Context).Done()
		}).Return(nil, context.DeadlineExceeded).Once()
		start := time.Now()
		_, err := service.TestRepository(t.Context(), &apiclient.TestRepositoryRequest{
			Repo: &v1alpha1.Repository{Repo: "https://charts.example.test", Type: "helm"},
		})
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Equal(t, time.Minute, time.Since(start))
	})
}

func TestTestRepositoryCanceledHelmCheckDoesNotStart(t *testing.T) {
	service, _ := newHelmRepositoryTestService(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := service.TestRepository(ctx, &apiclient.TestRepositoryRequest{
		Repo: &v1alpha1.Repository{Repo: "https://charts.example.test", Type: "helm"},
	})
	require.ErrorIs(t, err, context.Canceled)
}

func TestTestRepositoryHelmAndOCIUseSeparateChecks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service, client := newHelmRepositoryTestService(t)
		release := make(chan struct{})
		client.On("GetIndex", mock.Anything, false, int64(1024)).Run(func(mock.Arguments) {
			<-release
		}).Return(&helm.Index{}, nil).Once()
		client.On("TestHelmOCI", mock.Anything).Run(func(mock.Arguments) {
			<-release
		}).Return(true, nil).Once()
		results := make(chan error, 2)
		for _, enableOCI := range []bool{false, true} {
			go func() {
				_, err := service.TestRepository(t.Context(), &apiclient.TestRepositoryRequest{
					Repo: &v1alpha1.Repository{Repo: "charts.example.test", Type: "helm", EnableOCI: enableOCI},
				})
				results <- err
			}()
		}
		synctest.Wait()
		close(release)
		require.NoError(t, <-results)
		require.NoError(t, <-results)
	})
}

func TestTestRepositoryHelmChecksIsolateConfiguration(t *testing.T) {
	changes := map[string]func(*v1alpha1.Repository){
		"url":               func(r *v1alpha1.Repository) { r.Repo = "https://other.example.test" },
		"username":          func(r *v1alpha1.Repository) { r.Username = "other-user" },
		"password":          func(r *v1alpha1.Repository) { r.Password = "other-password" },
		"certificate":       func(r *v1alpha1.Repository) { r.TLSClientCertData = "other-cert" },
		"private key":       func(r *v1alpha1.Repository) { r.TLSClientCertKey = "other-key" },
		"insecure":          func(r *v1alpha1.Repository) { r.Insecure = true },
		"proxy":             func(r *v1alpha1.Repository) { r.Proxy = "http://proxy.example.test" },
		"no proxy":          func(r *v1alpha1.Repository) { r.NoProxy = "charts.example.test" },
		"workload identity": func(r *v1alpha1.Repository) { r.UseAzureWorkloadIdentity = true },
		"plain http":        func(r *v1alpha1.Repository) { r.InsecureOCIForceHttp = true },
		"project":           func(r *v1alpha1.Repository) { r.Project = "other-project" },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				service, client := newHelmRepositoryTestService(t)
				release := make(chan struct{})
				client.On("GetIndex", mock.Anything, false, int64(1024)).Run(func(mock.Arguments) {
					<-release
				}).Return(&helm.Index{}, nil).Twice()
				repo := &v1alpha1.Repository{Repo: "https://charts.example.test", Type: "helm", Password: "first-password"}
				other := repo.DeepCopy()
				change(other)
				results := make(chan error, 2)
				for _, r := range []*v1alpha1.Repository{repo, other} {
					go func() {
						_, err := service.TestRepository(t.Context(), &apiclient.TestRepositoryRequest{Repo: r})
						results <- err
					}()
				}
				synctest.Wait()
				close(release)
				require.NoError(t, <-results)
				require.NoError(t, <-results)
			})
		})
	}
}
