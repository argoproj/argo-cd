package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Masterminds/semver"
	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	"github.com/argoproj/argo-cd/reposerver/cache"
	"github.com/argoproj/argo-cd/reposerver/metrics"
	fileutil "github.com/argoproj/argo-cd/test/fixture/path"
	cacheutil "github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/git"
	gitmocks "github.com/argoproj/argo-cd/util/git/mocks"
	"github.com/argoproj/argo-cd/util/helm"
	helmmocks "github.com/argoproj/argo-cd/util/helm/mocks"
	"github.com/argoproj/argo-cd/util/io"
)

const testSignature = `gpg: Signature made Wed Feb 26 23:22:34 2020 CET
gpg:                using RSA key 4AEE18F83AFDEB23
gpg: Good signature from "GitHub (web-flow commit signing) <noreply@github.com>" [ultimate]
`

type clientFunc func(*gitmocks.Client)

func newServiceWithMocks(root string, signed bool) (*Service, *gitmocks.Client) {
	root, err := filepath.Abs(root)
	if err != nil {
		panic(err)
	}
	return newServiceWithOpt(func(gitClient *gitmocks.Client) {
		gitClient.On("Init").Return(nil)
		gitClient.On("Fetch").Return(nil)
		gitClient.On("Checkout", mock.Anything).Return(nil)
		gitClient.On("LsRemote", mock.Anything).Return(mock.Anything, nil)
		gitClient.On("CommitSHA").Return(mock.Anything, nil)
		gitClient.On("Root").Return(root)
		if signed {
			gitClient.On("VerifyCommitSignature", mock.Anything).Return(testSignature, nil)
		} else {
			gitClient.On("VerifyCommitSignature", mock.Anything).Return("", nil)
		}
	})
}

func newServiceWithOpt(cf clientFunc) (*Service, *gitmocks.Client) {
	// root, err := filepath.Abs(root)
	// if err != nil {
	// 	panic(err)
	// }
	helmClient := &helmmocks.Client{}
	gitClient := &gitmocks.Client{}
	cf(gitClient)
	service := NewService(metrics.NewMetricsServer(), cache.NewCache(
		cacheutil.NewCache(cacheutil.NewInMemoryCache(1*time.Minute)),
		1*time.Minute,
	), RepoServerInitConstants{ParallelismLimit: 1})

	chart := "my-chart"
	version := semver.MustParse("1.1.0")
	helmClient.On("GetIndex").Return(&helm.Index{Entries: map[string]helm.Entries{
		chart: {{Version: "1.0.0"}, {Version: version.String()}},
	}}, nil)
	helmClient.On("ExtractChart", chart, version).Return("./testdata/my-chart", io.NopCloser, nil)
	helmClient.On("CleanChartCache", chart, version).Return(nil)

	service.newGitClient = func(rawRepoURL string, creds git.Creds, insecure bool, enableLfs bool) (client git.Client, e error) {
		return gitClient, nil
	}
	service.newHelmClient = func(repoURL string, creds helm.Creds, enableOci bool) helm.Client {
		return helmClient
	}
	return service, gitClient
}

func newService(root string) *Service {
	service, _ := newServiceWithMocks(root, false)
	return service
}

func newServiceWithSignature(root string) *Service {
	service, _ := newServiceWithMocks(root, true)
	return service
}

func newServiceWithCommitSHA(root, revision string) *Service {
	var revisionErr error

	commitSHARegex := regexp.MustCompile("^[0-9A-Fa-f]{40}$")
	if !commitSHARegex.MatchString(revision) {
		revisionErr = errors.New("not a commit SHA")
	}

	service, gitClient := newServiceWithOpt(func(gitClient *gitmocks.Client) {
		gitClient.On("Init").Return(nil)
		gitClient.On("Fetch").Return(nil)
		gitClient.On("Checkout", mock.Anything).Return(nil)
		gitClient.On("LsRemote", revision).Return(revision, revisionErr)
		gitClient.On("CommitSHA").Return("632039659e542ed7de0c170a4fcc1c571b288fc0", nil)
		gitClient.On("Root").Return(root)
	})

	service.newGitClient = func(rawRepoURL string, creds git.Creds, insecure bool, enableLfs bool) (client git.Client, e error) {
		return gitClient, nil
	}

	return service
}

func TestGenerateYamlManifestInDir(t *testing.T) {
	service := newService("../..")

	src := argoappv1.ApplicationSource{Path: "manifests/base"}
	q := apiclient.ManifestRequest{Repo: &argoappv1.Repository{}, ApplicationSource: &src}

	// update this value if we add/remove manifests
	const countOfManifests = 29

	res1, err := service.GenerateManifest(context.Background(), &q)

	assert.NoError(t, err)
	assert.Equal(t, countOfManifests, len(res1.Manifests))

	// this will test concatenated manifests to verify we split YAMLs correctly
	res2, err := GenerateManifests("./testdata/concatenated", "/", "", &q, false)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(res2.Manifests))
}

// ensure we can use a semver constraint range (>= 1.0.0) and get back the correct chart (1.0.0)
func TestHelmManifestFromChartRepo(t *testing.T) {
	service := newService(".")
	source := &argoappv1.ApplicationSource{Chart: "my-chart", TargetRevision: ">= 1.0.0"}
	request := &apiclient.ManifestRequest{Repo: &argoappv1.Repository{}, ApplicationSource: source, NoCache: true}
	response, err := service.GenerateManifest(context.Background(), request)
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, &apiclient.ManifestResponse{
		Manifests:  []string{"{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"my-map\"}}"},
		Namespace:  "",
		Server:     "",
		Revision:   "1.1.0",
		SourceType: "Helm",
	}, response)
}

func TestGenerateManifestsUseExactRevision(t *testing.T) {
	service, gitClient := newServiceWithMocks(".", false)

	src := argoappv1.ApplicationSource{Path: "./testdata/recurse", Directory: &argoappv1.ApplicationSourceDirectory{Recurse: true}}

	q := apiclient.ManifestRequest{Repo: &argoappv1.Repository{}, ApplicationSource: &src, Revision: "abc"}

	res1, err := service.GenerateManifest(context.Background(), &q)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res1.Manifests))
	assert.Equal(t, gitClient.Calls[0].Arguments[0], "abc")
}

func TestRecurseManifestsInDir(t *testing.T) {
	service := newService(".")

	src := argoappv1.ApplicationSource{Path: "./testdata/recurse", Directory: &argoappv1.ApplicationSourceDirectory{Recurse: true}}

	q := apiclient.ManifestRequest{Repo: &argoappv1.Repository{}, ApplicationSource: &src}

	res1, err := service.GenerateManifest(context.Background(), &q)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res1.Manifests))
}

func TestGenerateJsonnetManifestInDir(t *testing.T) {
	service := newService(".")

	q := apiclient.ManifestRequest{
		Repo: &argoappv1.Repository{},
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./testdata/jsonnet",
			Directory: &argoappv1.ApplicationSourceDirectory{
				Jsonnet: argoappv1.ApplicationSourceJsonnet{
					ExtVars: []argoappv1.JsonnetVar{{Name: "extVarString", Value: "extVarString"}, {Name: "extVarCode", Value: "\"extVarCode\"", Code: true}},
					TLAs:    []argoappv1.JsonnetVar{{Name: "tlaString", Value: "tlaString"}, {Name: "tlaCode", Value: "\"tlaCode\"", Code: true}},
					Libs:    []string{"testdata/jsonnet/vendor"},
				},
			},
		},
	}
	res1, err := service.GenerateManifest(context.Background(), &q)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res1.Manifests))
}

func TestGenerateKsonnetManifest(t *testing.T) {
	service := newService("../..")

	q := apiclient.ManifestRequest{
		Repo: &argoappv1.Repository{},
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./test/e2e/testdata/ksonnet",
			Ksonnet: &argoappv1.ApplicationSourceKsonnet{
				Environment: "dev",
			},
		},
	}
	res, err := service.GenerateManifest(context.Background(), &q)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res.Manifests))
	assert.Equal(t, "dev", res.Namespace)
	assert.Equal(t, "https://kubernetes.default.svc", res.Server)
}

func TestGenerateHelmChartWithDependencies(t *testing.T) {
	service := newService("../..")

	q := apiclient.ManifestRequest{
		Repo: &argoappv1.Repository{},
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./util/helm/testdata/helm2-dependency",
		},
	}
	res1, err := service.GenerateManifest(context.Background(), &q)
	assert.Nil(t, err)
	assert.Len(t, res1.Manifests, 12)
}

func TestManifestGenErrorCacheByNumRequests(t *testing.T) {

	// Returns the state of the manifest generation cache, by querying the cache for the previously set result
	getRecentCachedEntry := func(service *Service, manifestRequest *apiclient.ManifestRequest) *cache.CachedManifestResponse {
		assert.NotNil(t, service)
		assert.NotNil(t, manifestRequest)

		cachedManifestResponse := &cache.CachedManifestResponse{}
		err := service.cache.GetManifests(mock.Anything, manifestRequest.ApplicationSource, manifestRequest.Namespace, manifestRequest.AppLabelKey, manifestRequest.AppLabelValue, cachedManifestResponse)
		assert.Nil(t, err)
		return cachedManifestResponse
	}

	// Example:
	// With repo server (test) parameters:
	// - PauseGenerationAfterFailedGenerationAttempts: 2
	// - PauseGenerationOnFailureForRequests: 4
	// - TotalCacheInvocations: 10
	//
	// After 2 manifest generation failures in a row, the next 4 manifest generation requests should be cached,
	// with the next 2 after that being uncached. Here's how it looks...
	//
	//  request count) result
	// --------------------------
	// 1) Attempt to generate manifest, fails.
	// 2) Second attempt to generate manifest, fails.
	// 3) Return cached error attempt from #2
	// 4) Return cached error attempt from #2
	// 5) Return cached error attempt from #2
	// 6) Return cached error attempt from #2. Max response limit hit, so reset cache entry.
	// 7) Attempt to generate manifest, fails.
	// 8) Attempt to generate manifest, fails.
	// 9) Return cached error attempt from #8
	// 10) Return cached error attempt from #8

	// The same pattern PauseGenerationAfterFailedGenerationAttempts generation attempts, followed by
	// PauseGenerationOnFailureForRequests cached responses, should apply for various combinations of
	// both parameters.

	tests := []struct {
		PauseGenerationAfterFailedGenerationAttempts int
		PauseGenerationOnFailureForRequests          int
		TotalCacheInvocations                        int
	}{
		{2, 4, 10},
		{3, 5, 10},
		{1, 2, 5},
	}
	for _, tt := range tests {
		testName := fmt.Sprintf("gen-attempts-%d-pause-%d-total-%d", tt.PauseGenerationAfterFailedGenerationAttempts, tt.PauseGenerationOnFailureForRequests, tt.TotalCacheInvocations)
		t.Run(testName, func(t *testing.T) {
			service := newService(".")

			service.initConstants = RepoServerInitConstants{
				ParallelismLimit: 1,
				PauseGenerationAfterFailedGenerationAttempts: tt.PauseGenerationAfterFailedGenerationAttempts,
				PauseGenerationOnFailureForMinutes:           0,
				PauseGenerationOnFailureForRequests:          tt.PauseGenerationOnFailureForRequests,
			}

			totalAttempts := service.initConstants.PauseGenerationAfterFailedGenerationAttempts + service.initConstants.PauseGenerationOnFailureForRequests

			for invocationCount := 0; invocationCount < tt.TotalCacheInvocations; invocationCount++ {
				adjustedInvocation := invocationCount % totalAttempts

				fmt.Printf("%d )-------------------------------------------\n", invocationCount)

				manifestRequest := &apiclient.ManifestRequest{
					Repo:          &argoappv1.Repository{},
					AppLabelValue: "test",
					ApplicationSource: &argoappv1.ApplicationSource{
						Path: "./testdata/invalid-helm",
					},
				}

				res, err := service.GenerateManifest(context.Background(), manifestRequest)

				// Verify invariant: res != nil xor err != nil
				if err != nil {
					assert.True(t, res == nil, "both err and res are non-nil res: %v   err: %v", res, err)
				} else {
					assert.True(t, res != nil, "both err and res are nil")
				}

				cachedManifestResponse := getRecentCachedEntry(service, manifestRequest)

				isCachedError := err != nil && strings.HasPrefix(err.Error(), cachedManifestGenerationPrefix)

				if adjustedInvocation < service.initConstants.PauseGenerationAfterFailedGenerationAttempts {
					// GenerateManifest should not return cached errors for the first X responses, where X is the FailGenAttempts constants
					assert.True(t, !isCachedError)

					assert.True(t, cachedManifestResponse != nil)
					// nolint:staticcheck
					assert.True(t, cachedManifestResponse.ManifestResponse == nil)
					// nolint:staticcheck
					assert.True(t, cachedManifestResponse.FirstFailureTimestamp != 0)

					// Internal cache consec failures value should increase with invocations, cached response should stay the same,
					// nolint:staticcheck
					assert.True(t, cachedManifestResponse.NumberOfConsecutiveFailures == adjustedInvocation+1)
					// nolint:staticcheck
					assert.True(t, cachedManifestResponse.NumberOfCachedResponsesReturned == 0)

				} else {
					// GenerateManifest SHOULD return cached errors for the next X responses, where X is the
					// PauseGenerationOnFailureForRequests constant
					assert.True(t, isCachedError)
					assert.True(t, cachedManifestResponse != nil)
					// nolint:staticcheck
					assert.True(t, cachedManifestResponse.ManifestResponse == nil)
					// nolint:staticcheck
					assert.True(t, cachedManifestResponse.FirstFailureTimestamp != 0)

					// Internal cache values should update correctly based on number of return cache entries, consecutive failures should stay the same
					assert.True(t, cachedManifestResponse.NumberOfConsecutiveFailures == service.initConstants.PauseGenerationAfterFailedGenerationAttempts)
					assert.True(t, cachedManifestResponse.NumberOfCachedResponsesReturned == (adjustedInvocation-service.initConstants.PauseGenerationAfterFailedGenerationAttempts+1))
				}
			}
		})
	}
}

func TestManifestGenErrorCacheFileContentsChange(t *testing.T) {

	tmpDir, err := ioutil.TempDir("", "repository-test-")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	service := newService(tmpDir)

	service.initConstants = RepoServerInitConstants{
		ParallelismLimit: 1,
		PauseGenerationAfterFailedGenerationAttempts: 2,
		PauseGenerationOnFailureForMinutes:           0,
		PauseGenerationOnFailureForRequests:          4,
	}

	for step := 0; step < 3; step++ {

		// step 1) Attempt to generate manifests against invalid helm chart (should return uncached error)
		// step 2) Attempt to generate manifest against valid helm chart (should succeed and return valid response)
		// step 3) Attempt to generate manifest against invalid helm chart (should return cached value from step 2)

		errorExpected := step%2 == 0

		// Ensure that the target directory will succeed or fail, so we can verify the cache correctly handles it
		err = os.RemoveAll(tmpDir)
		assert.NoError(t, err)
		err = os.MkdirAll(tmpDir, 0777)
		assert.NoError(t, err)
		if errorExpected {
			// Copy invalid helm chart into temporary directory, ensuring manifest generation will fail
			err = fileutil.CopyDir("./testdata/invalid-helm", tmpDir)
			assert.NoError(t, err)

		} else {
			// Copy valid helm chart into temporary directory, ensuring generation will succeed
			err = fileutil.CopyDir("./testdata/my-chart", tmpDir)
			assert.NoError(t, err)
		}

		res, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
			Repo:          &argoappv1.Repository{},
			AppLabelValue: "test",
			ApplicationSource: &argoappv1.ApplicationSource{
				Path: ".",
			},
		})

		fmt.Println("-", step, "-", res != nil, err != nil, errorExpected)
		fmt.Println("    err: ", err)
		fmt.Println("    res: ", res)

		if step < 2 {
			assert.True(t, (err != nil) == errorExpected, "error return value and error expected did not match")
			assert.True(t, (res != nil) == !errorExpected, "GenerateManifest return value and expected value did not match")
		}

		if step == 2 {
			assert.True(t, err == nil, "error ret val was non-nil on step 3")
			assert.True(t, res != nil, "GenerateManifest ret val was nil on step 3")
		}
	}
}

func TestManifestGenErrorCacheByMinutesElapsed(t *testing.T) {

	tests := []struct {
		// Test with a range of pause expiration thresholds
		PauseGenerationOnFailureForMinutes int
	}{
		{1}, {2}, {10}, {24 * 60},
	}
	for _, tt := range tests {
		testName := fmt.Sprintf("pause-time-%d", tt.PauseGenerationOnFailureForMinutes)
		t.Run(testName, func(t *testing.T) {
			service := newService(".")

			// Here we simulate the passage of time by overriding the now() function of Service
			currentTime := time.Now()
			service.now = func() time.Time {
				return currentTime
			}

			service.initConstants = RepoServerInitConstants{
				ParallelismLimit: 1,
				PauseGenerationAfterFailedGenerationAttempts: 1,
				PauseGenerationOnFailureForMinutes:           tt.PauseGenerationOnFailureForMinutes,
				PauseGenerationOnFailureForRequests:          0,
			}

			// 1) Put the cache into the failure state
			for x := 0; x < 2; x++ {
				res, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
					Repo:          &argoappv1.Repository{},
					AppLabelValue: "test",
					ApplicationSource: &argoappv1.ApplicationSource{
						Path: "./testdata/invalid-helm",
					},
				})

				assert.True(t, err != nil && res == nil)

				// Ensure that the second invocation triggers the cached error state
				if x == 1 {
					assert.True(t, strings.HasPrefix(err.Error(), cachedManifestGenerationPrefix))
				}

			}

			// 2) Jump forward X-1 minutes in time, where X is the expiration boundary
			currentTime = currentTime.Add(time.Duration(tt.PauseGenerationOnFailureForMinutes-1) * time.Minute)
			res, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
				Repo:          &argoappv1.Repository{},
				AppLabelValue: "test",
				ApplicationSource: &argoappv1.ApplicationSource{
					Path: "./testdata/invalid-helm",
				},
			})

			// 3) Ensure that the cache still returns a cached copy of the last error
			assert.True(t, err != nil && res == nil)
			assert.True(t, strings.HasPrefix(err.Error(), cachedManifestGenerationPrefix))

			// 4) Jump forward 2 minutes in time, such that the pause generation time has elapsed and we should return to normal state
			currentTime = currentTime.Add(2 * time.Minute)

			res, err = service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
				Repo:          &argoappv1.Repository{},
				AppLabelValue: "test",
				ApplicationSource: &argoappv1.ApplicationSource{
					Path: "./testdata/invalid-helm",
				},
			})

			// 5) Ensure that the service no longer returns a cached copy of the last error
			assert.True(t, err != nil && res == nil)
			assert.True(t, !strings.HasPrefix(err.Error(), cachedManifestGenerationPrefix))

		})
	}

}

func TestManifestGenErrorCacheRespectsNoCache(t *testing.T) {

	service := newService(".")

	service.initConstants = RepoServerInitConstants{
		ParallelismLimit: 1,
		PauseGenerationAfterFailedGenerationAttempts: 1,
		PauseGenerationOnFailureForMinutes:           0,
		PauseGenerationOnFailureForRequests:          4,
	}

	// 1) Put the cache into the failure state
	for x := 0; x < 2; x++ {
		res, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
			Repo:          &argoappv1.Repository{},
			AppLabelValue: "test",
			ApplicationSource: &argoappv1.ApplicationSource{
				Path: "./testdata/invalid-helm",
			},
		})

		assert.True(t, err != nil && res == nil)

		// Ensure that the second invocation is cached
		if x == 1 {
			assert.True(t, strings.HasPrefix(err.Error(), cachedManifestGenerationPrefix))
		}
	}

	// 2) Call generateManifest with NoCache enabled
	res, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:          &argoappv1.Repository{},
		AppLabelValue: "test",
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./testdata/invalid-helm",
		},
		NoCache: true,
	})

	// 3) Ensure that the cache returns a new generation attempt, rather than a previous cached error
	assert.True(t, err != nil && res == nil)
	assert.True(t, !strings.HasPrefix(err.Error(), cachedManifestGenerationPrefix))

	// 4) Call generateManifest
	res, err = service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:          &argoappv1.Repository{},
		AppLabelValue: "test",
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./testdata/invalid-helm",
		},
	})

	// 5) Ensure that the subsequent invocation, after nocache, is cached
	assert.True(t, err != nil && res == nil)
	assert.True(t, strings.HasPrefix(err.Error(), cachedManifestGenerationPrefix))

}

func TestGenerateHelmWithValues(t *testing.T) {
	service := newService("../..")

	res, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:          &argoappv1.Repository{},
		AppLabelValue: "test",
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./util/helm/testdata/redis",
			Helm: &argoappv1.ApplicationSourceHelm{
				ValueFiles: []string{"values-production.yaml"},
				Values:     `cluster: {slaveCount: 2}`,
			},
		},
	})

	assert.NoError(t, err)

	replicasVerified := false
	for _, src := range res.Manifests {
		obj := unstructured.Unstructured{}
		err = json.Unmarshal([]byte(src), &obj)
		assert.NoError(t, err)

		if obj.GetKind() == "Deployment" && obj.GetName() == "test-redis-slave" {
			var dep v1.Deployment
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &dep)
			assert.NoError(t, err)
			assert.Equal(t, int32(2), *dep.Spec.Replicas)
			replicasVerified = true
		}
	}
	assert.True(t, replicasVerified)

}

// The requested value file (`../minio/values.yaml`) is outside the app path (`./util/helm/testdata/redis`), however
// since the requested value is sill under the repo directory (`~/go/src/github.com/argoproj/argo-cd`), it is allowed
func TestGenerateHelmWithValuesDirectoryTraversal(t *testing.T) {
	service := newService("../..")
	_, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:          &argoappv1.Repository{},
		AppLabelValue: "test",
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./util/helm/testdata/redis",
			Helm: &argoappv1.ApplicationSourceHelm{
				ValueFiles: []string{"../minio/values.yaml"},
				Values:     `cluster: {slaveCount: 2}`,
			},
		},
	})
	assert.NoError(t, err)

	// Test the case where the path is "."
	service = newService("./testdata/my-chart")
	_, err = service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:          &argoappv1.Repository{},
		AppLabelValue: "test",
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: ".",
		},
	})
	assert.NoError(t, err)
}

// This is a Helm first-class app with a values file inside the repo directory
// (`~/go/src/github.com/argoproj/argo-cd/reposerver/repository`), so it is allowed
func TestHelmManifestFromChartRepoWithValueFile(t *testing.T) {
	service := newService(".")
	source := &argoappv1.ApplicationSource{
		Chart:          "my-chart",
		TargetRevision: ">= 1.0.0",
		Helm: &argoappv1.ApplicationSourceHelm{
			ValueFiles: []string{"./my-chart-values.yaml"},
		},
	}
	request := &apiclient.ManifestRequest{Repo: &argoappv1.Repository{}, ApplicationSource: source, NoCache: true}
	response, err := service.GenerateManifest(context.Background(), request)
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, &apiclient.ManifestResponse{
		Manifests:  []string{"{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"my-map\"}}"},
		Namespace:  "",
		Server:     "",
		Revision:   "1.1.0",
		SourceType: "Helm",
	}, response)
}

// This is a Helm first-class app with a values file outside the repo directory
// (`~/go/src/github.com/argoproj/argo-cd/reposerver/repository`), so it is not allowed
func TestHelmManifestFromChartRepoWithValueFileOutsideRepo(t *testing.T) {
	service := newService(".")
	source := &argoappv1.ApplicationSource{
		Chart:          "my-chart",
		TargetRevision: ">= 1.0.0",
		Helm: &argoappv1.ApplicationSourceHelm{
			ValueFiles: []string{"../my-chart-2/my-chart-2-values.yaml"},
		},
	}
	request := &apiclient.ManifestRequest{Repo: &argoappv1.Repository{}, ApplicationSource: source, NoCache: true}
	_, err := service.GenerateManifest(context.Background(), request)
	assert.Error(t, err, "should be on or under current directory")
}

func TestGenerateHelmWithURL(t *testing.T) {
	service := newService("../..")

	_, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:          &argoappv1.Repository{},
		AppLabelValue: "test",
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./util/helm/testdata/redis",
			Helm: &argoappv1.ApplicationSourceHelm{
				ValueFiles: []string{"https://raw.githubusercontent.com/argoproj/argocd-example-apps/master/helm-guestbook/values.yaml"},
				Values:     `cluster: {slaveCount: 2}`,
			},
		},
	})
	assert.NoError(t, err)
}

// The requested value file (`../../../../../minio/values.yaml`) is outside the repo directory
// (`~/go/src/github.com/argoproj/argo-cd`), so it is blocked
func TestGenerateHelmWithValuesDirectoryTraversalOutsideRepo(t *testing.T) {
	service := newService("../..")
	_, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:          &argoappv1.Repository{},
		AppLabelValue: "test",
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./util/helm/testdata/redis",
			Helm: &argoappv1.ApplicationSourceHelm{
				ValueFiles: []string{"../../../../../minio/values.yaml"},
				Values:     `cluster: {slaveCount: 2}`,
			},
		},
	})
	assert.Error(t, err, "should be on or under current directory")

	service = newService("./testdata/my-chart")
	_, err = service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:          &argoappv1.Repository{},
		AppLabelValue: "test",
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: ".",
			Helm: &argoappv1.ApplicationSourceHelm{
				ValueFiles: []string{"../my-chart-2/values.yaml"},
				Values:     `cluster: {slaveCount: 2}`,
			},
		},
	})
	assert.Error(t, err, "should be on or under current directory")
}

// The requested file parameter (`/tmp/external-secret.txt`) is outside the app path
// (`./util/helm/testdata/redis`), and outside the repo directory. It is used as a means
// of providing direct content to a helm chart via a specific key.
func TestGenerateHelmWithAbsoluteFileParameter(t *testing.T) {
	service := newService("../..")

	file, err := ioutil.TempFile("", "external-secret.txt")
	assert.NoError(t, err)
	externalSecretPath := file.Name()
	defer func() { _ = os.RemoveAll(externalSecretPath) }()
	expectedFileContent, err := ioutil.ReadFile("../../util/helm/testdata/external/external-secret.txt")
	assert.NoError(t, err)
	err = ioutil.WriteFile(externalSecretPath, expectedFileContent, 0644)
	assert.NoError(t, err)

	_, err = service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:          &argoappv1.Repository{},
		AppLabelValue: "test",
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./util/helm/testdata/redis",
			Helm: &argoappv1.ApplicationSourceHelm{
				ValueFiles: []string{"values-production.yaml"},
				Values:     `cluster: {slaveCount: 2}`,
				FileParameters: []argoappv1.HelmFileParameter{
					argoappv1.HelmFileParameter{
						Name: "passwordContent",
						Path: externalSecretPath,
					},
				},
			},
		},
	})
	assert.NoError(t, err)
}

// The requested file parameter (`../external/external-secret.txt`) is outside the app path
// (`./util/helm/testdata/redis`), however  since the requested value is sill under the repo
// directory (`~/go/src/github.com/argoproj/argo-cd`), it is allowed. It is used as a means of
// providing direct content to a helm chart via a specific key.
func TestGenerateHelmWithFileParameter(t *testing.T) {
	service := newService("../..")

	_, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:          &argoappv1.Repository{},
		AppLabelValue: "test",
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./util/helm/testdata/redis",
			Helm: &argoappv1.ApplicationSourceHelm{
				ValueFiles: []string{"values-production.yaml"},
				Values:     `cluster: {slaveCount: 2}`,
				FileParameters: []argoappv1.HelmFileParameter{
					argoappv1.HelmFileParameter{
						Name: "passwordContent",
						Path: "../external/external-secret.txt",
					},
				},
			},
		},
	})
	assert.NoError(t, err)
}

func TestGenerateNullList(t *testing.T) {
	service := newService(".")

	res1, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:              &argoappv1.Repository{},
		ApplicationSource: &argoappv1.ApplicationSource{Path: "./testdata/null-list"},
	})
	assert.Nil(t, err)
	assert.Equal(t, len(res1.Manifests), 1)
	assert.Contains(t, res1.Manifests[0], "prometheus-operator-operator")

	res1, err = service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:              &argoappv1.Repository{},
		ApplicationSource: &argoappv1.ApplicationSource{Path: "./testdata/empty-list"},
	})
	assert.Nil(t, err)
	assert.Equal(t, len(res1.Manifests), 1)
	assert.Contains(t, res1.Manifests[0], "prometheus-operator-operator")

	res1, err = service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:              &argoappv1.Repository{},
		ApplicationSource: &argoappv1.ApplicationSource{Path: "./testdata/weird-list"},
	})
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res1.Manifests))
}

func TestIdentifyAppSourceTypeByAppDirWithKustomizations(t *testing.T) {
	sourceType, err := GetAppSourceType(&argoappv1.ApplicationSource{}, "./testdata/kustomization_yaml")
	assert.Nil(t, err)
	assert.Equal(t, argoappv1.ApplicationSourceTypeKustomize, sourceType)

	sourceType, err = GetAppSourceType(&argoappv1.ApplicationSource{}, "./testdata/kustomization_yml")
	assert.Nil(t, err)
	assert.Equal(t, argoappv1.ApplicationSourceTypeKustomize, sourceType)

	sourceType, err = GetAppSourceType(&argoappv1.ApplicationSource{}, "./testdata/Kustomization")
	assert.Nil(t, err)
	assert.Equal(t, argoappv1.ApplicationSourceTypeKustomize, sourceType)
}

func TestRunCustomTool(t *testing.T) {
	service := newService(".")

	res, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		AppLabelValue: "test-app",
		Namespace:     "test-namespace",
		ApplicationSource: &argoappv1.ApplicationSource{
			Plugin: &argoappv1.ApplicationSourcePlugin{
				Name: "test",
			},
		},
		Plugins: []*argoappv1.ConfigManagementPlugin{{
			Name: "test",
			Generate: argoappv1.Command{
				Command: []string{"sh", "-c"},
				Args:    []string{`echo "{\"kind\": \"FakeObject\", \"metadata\": { \"name\": \"$ARGOCD_APP_NAME\", \"namespace\": \"$ARGOCD_APP_NAMESPACE\", \"annotations\": {\"GIT_ASKPASS\": \"$GIT_ASKPASS\", \"GIT_USERNAME\": \"$GIT_USERNAME\", \"GIT_PASSWORD\": \"$GIT_PASSWORD\"}}}"`},
			},
		}},
		Repo: &argoappv1.Repository{
			Username: "foo", Password: "bar",
		},
	})

	assert.Nil(t, err)
	assert.Equal(t, 1, len(res.Manifests))

	obj := &unstructured.Unstructured{}
	assert.Nil(t, json.Unmarshal([]byte(res.Manifests[0]), obj))

	assert.Equal(t, obj.GetName(), "test-app")
	assert.Equal(t, obj.GetNamespace(), "test-namespace")
	assert.Equal(t, "git-ask-pass.sh", obj.GetAnnotations()["GIT_ASKPASS"])
	assert.Equal(t, "foo", obj.GetAnnotations()["GIT_USERNAME"])
	assert.Equal(t, "bar", obj.GetAnnotations()["GIT_PASSWORD"])
}

func TestGenerateFromUTF16(t *testing.T) {
	q := apiclient.ManifestRequest{
		Repo:              &argoappv1.Repository{},
		ApplicationSource: &argoappv1.ApplicationSource{},
	}
	res1, err := GenerateManifests("./testdata/utf-16", "/", "", &q, false)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res1.Manifests))
}

func TestListApps(t *testing.T) {
	service := newService("./testdata")

	res, err := service.ListApps(context.Background(), &apiclient.ListAppsRequest{Repo: &argoappv1.Repository{}})
	assert.NoError(t, err)

	expectedApps := map[string]string{
		"Kustomization":      "Kustomize",
		"app-parameters":     "Kustomize",
		"invalid-helm":       "Helm",
		"invalid-kustomize":  "Kustomize",
		"kustomization_yaml": "Kustomize",
		"kustomization_yml":  "Kustomize",
		"my-chart":           "Helm",
		"my-chart-2":         "Helm",
	}
	assert.Equal(t, expectedApps, res.Apps)
}

func TestGetAppDetailsHelm(t *testing.T) {
	service := newService("../..")

	res, err := service.GetAppDetails(context.Background(), &apiclient.RepoServerAppDetailsQuery{
		Repo: &argoappv1.Repository{},
		Source: &argoappv1.ApplicationSource{
			Path: "./util/helm/testdata/helm2-dependency",
		},
	})

	assert.NoError(t, err)
	assert.NotNil(t, res.Helm)

	assert.Equal(t, "Helm", res.Type)
	assert.EqualValues(t, []string{"values-production.yaml", "values.yaml"}, res.Helm.ValueFiles)
}

func TestGetAppDetailsKustomize(t *testing.T) {
	service := newService("../..")

	res, err := service.GetAppDetails(context.Background(), &apiclient.RepoServerAppDetailsQuery{
		Repo: &argoappv1.Repository{},
		Source: &argoappv1.ApplicationSource{
			Path: "./util/kustomize/testdata/kustomization_yaml",
		},
	})

	assert.NoError(t, err)

	assert.Equal(t, "Kustomize", res.Type)
	assert.NotNil(t, res.Kustomize)
	assert.EqualValues(t, []string{"nginx:1.15.4", "k8s.gcr.io/nginx-slim:0.8"}, res.Kustomize.Images)
}

func TestGetAppDetailsKsonnet(t *testing.T) {
	service := newService("../..")

	res, err := service.GetAppDetails(context.Background(), &apiclient.RepoServerAppDetailsQuery{
		Repo: &argoappv1.Repository{},
		Source: &argoappv1.ApplicationSource{
			Path: "./test/e2e/testdata/ksonnet",
		},
	})

	assert.NoError(t, err)

	assert.Equal(t, "Ksonnet", res.Type)
	assert.NotNil(t, res.Ksonnet)
	assert.Equal(t, "guestbook", res.Ksonnet.Name)
	assert.Len(t, res.Ksonnet.Environments, 3)
}

func TestGetHelmCharts(t *testing.T) {
	service := newService("../..")
	res, err := service.GetHelmCharts(context.Background(), &apiclient.HelmChartsRequest{Repo: &argoappv1.Repository{}})
	assert.NoError(t, err)
	assert.Len(t, res.Items, 1)

	item := res.Items[0]
	assert.Equal(t, "my-chart", item.Name)
	assert.EqualValues(t, []string{"1.0.0", "1.1.0"}, item.Versions)
}

func TestGetRevisionMetadata(t *testing.T) {
	service, gitClient := newServiceWithMocks("../..", false)
	now := time.Now()

	gitClient.On("RevisionMetadata", mock.Anything).Return(&git.RevisionMetadata{
		Message: strings.Repeat("a", 100) + "\n" + "second line",
		Author:  "author",
		Date:    now,
		Tags:    []string{"tag1", "tag2"},
	}, nil)

	res, err := service.GetRevisionMetadata(context.Background(), &apiclient.RepoServerRevisionMetadataRequest{
		Repo:     &argoappv1.Repository{},
		Revision: "c0b400fc458875d925171398f9ba9eabd5529923",
	})

	assert.NoError(t, err)
	assert.Equal(t, strings.Repeat("a", 61)+"...", res.Message)
	assert.Equal(t, now, res.Date.Time)
	assert.Equal(t, "author", res.Author)
	assert.EqualValues(t, []string{"tag1", "tag2"}, res.Tags)

}

func TestGetSignatureVerificationResult(t *testing.T) {
	// Commit with signature and verification requested
	{
		service := newServiceWithSignature("../..")

		src := argoappv1.ApplicationSource{Path: "manifests/base"}
		q := apiclient.ManifestRequest{Repo: &argoappv1.Repository{}, ApplicationSource: &src, VerifySignature: true}

		res, err := service.GenerateManifest(context.Background(), &q)
		assert.NoError(t, err)
		assert.Equal(t, testSignature, res.VerifyResult)
	}
	// Commit with signature and verification not requested
	{
		service := newServiceWithSignature("../..")

		src := argoappv1.ApplicationSource{Path: "manifests/base"}
		q := apiclient.ManifestRequest{Repo: &argoappv1.Repository{}, ApplicationSource: &src}

		res, err := service.GenerateManifest(context.Background(), &q)
		assert.NoError(t, err)
		assert.Empty(t, res.VerifyResult)
	}
	// Commit without signature and verification requested
	{
		service := newService("../..")

		src := argoappv1.ApplicationSource{Path: "manifests/base"}
		q := apiclient.ManifestRequest{Repo: &argoappv1.Repository{}, ApplicationSource: &src, VerifySignature: true}

		res, err := service.GenerateManifest(context.Background(), &q)
		assert.NoError(t, err)
		assert.Empty(t, res.VerifyResult)
	}
	// Commit without signature and verification not requested
	{
		service := newService("../..")

		src := argoappv1.ApplicationSource{Path: "manifests/base"}
		q := apiclient.ManifestRequest{Repo: &argoappv1.Repository{}, ApplicationSource: &src, VerifySignature: true}

		res, err := service.GenerateManifest(context.Background(), &q)
		assert.NoError(t, err)
		assert.Empty(t, res.VerifyResult)
	}
}

func Test_newEnv(t *testing.T) {
	assert.Equal(t, &argoappv1.Env{
		&argoappv1.EnvEntry{Name: "ARGOCD_APP_NAME", Value: "my-app-name"},
		&argoappv1.EnvEntry{Name: "ARGOCD_APP_NAMESPACE", Value: "my-namespace"},
		&argoappv1.EnvEntry{Name: "ARGOCD_APP_REVISION", Value: "my-revision"},
		&argoappv1.EnvEntry{Name: "ARGOCD_APP_SOURCE_REPO_URL", Value: "https://github.com/my-org/my-repo"},
		&argoappv1.EnvEntry{Name: "ARGOCD_APP_SOURCE_PATH", Value: "my-path"},
		&argoappv1.EnvEntry{Name: "ARGOCD_APP_SOURCE_TARGET_REVISION", Value: "my-target-revision"},
	}, newEnv(&apiclient.ManifestRequest{
		AppLabelValue: "my-app-name",
		Namespace:     "my-namespace",
		Repo:          &argoappv1.Repository{Repo: "https://github.com/my-org/my-repo"},
		ApplicationSource: &argoappv1.ApplicationSource{
			Path:           "my-path",
			TargetRevision: "my-target-revision",
		},
	}, "my-revision"))
}

func TestService_newHelmClientResolveRevision(t *testing.T) {
	service := newService(".")

	t.Run("EmptyRevision", func(t *testing.T) {
		_, _, err := service.newHelmClientResolveRevision(&argoappv1.Repository{}, "", "")
		assert.EqualError(t, err, "invalid revision '': improper constraint: ")
	})
	t.Run("InvalidRevision", func(t *testing.T) {
		_, _, err := service.newHelmClientResolveRevision(&argoappv1.Repository{}, "???", "")
		assert.EqualError(t, err, "invalid revision '???': improper constraint: ???")
	})
}

func TestGetAppDetailsWithAppParameterFile(t *testing.T) {
	service := newService(".")
	details, err := service.GetAppDetails(context.Background(), &apiclient.RepoServerAppDetailsQuery{
		Repo: &argoappv1.Repository{},
		Source: &argoappv1.ApplicationSource{
			Path: "./testdata/app-parameters",
		},
	})
	if !assert.NoError(t, err) {
		return
	}
	assert.EqualValues(t, []string{"gcr.io/heptio-images/ks-guestbook-demo:0.2"}, details.Kustomize.Images)
}

func TestGenerateManifestsWithAppParameterFile(t *testing.T) {
	service := newService(".")
	manifests, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo: &argoappv1.Repository{},
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./testdata/app-parameters",
		},
	})
	if !assert.NoError(t, err) {
		return
	}
	resourceByKindName := make(map[string]*unstructured.Unstructured)
	for _, manifest := range manifests.Manifests {
		var un unstructured.Unstructured
		err := yaml.Unmarshal([]byte(manifest), &un)
		if !assert.NoError(t, err) {
			return
		}
		resourceByKindName[fmt.Sprintf("%s/%s", un.GetKind(), un.GetName())] = &un
	}
	deployment, ok := resourceByKindName["Deployment/guestbook-ui"]
	if !assert.True(t, ok) {
		return
	}
	containers, ok, _ := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	if !assert.True(t, ok) {
		return
	}
	image, ok, _ := unstructured.NestedString(containers[0].(map[string]interface{}), "image")
	if !assert.True(t, ok) {
		return
	}
	assert.Equal(t, "gcr.io/heptio-images/ks-guestbook-demo:0.2", image)
}

func TestGenerateManifestWithAnnotatedAndRegularGitTagHashes(t *testing.T) {
	regularGitTagHash := "632039659e542ed7de0c170a4fcc1c571b288fc0"
	annotatedGitTaghash := "95249be61b028d566c29d47b19e65c5603388a41"
	invalidGitTaghash := "invalid-tag"
	actualCommitSHA := "632039659e542ed7de0c170a4fcc1c571b288fc0"

	tests := []struct {
		name            string
		ctx             context.Context
		manifestRequest *apiclient.ManifestRequest
		wantError       bool
		service         *Service
	}{
		{
			name: "Case: Git tag hash matches latest commit SHA (regular tag)",
			ctx:  context.Background(),
			manifestRequest: &apiclient.ManifestRequest{
				Repo: &argoappv1.Repository{},
				ApplicationSource: &argoappv1.ApplicationSource{
					TargetRevision: regularGitTagHash,
				},
				NoCache: true,
			},
			wantError: false,
			service:   newServiceWithCommitSHA(".", regularGitTagHash),
		},

		{
			name: "Case: Git tag hash does not match latest commit SHA (annotated tag)",
			ctx:  context.Background(),
			manifestRequest: &apiclient.ManifestRequest{
				Repo: &argoappv1.Repository{},
				ApplicationSource: &argoappv1.ApplicationSource{
					TargetRevision: annotatedGitTaghash,
				},
				NoCache: true,
			},
			wantError: false,
			service:   newServiceWithCommitSHA(".", annotatedGitTaghash),
		},

		{
			name: "Case: Git tag hash is invalid",
			ctx:  context.Background(),
			manifestRequest: &apiclient.ManifestRequest{
				Repo: &argoappv1.Repository{},
				ApplicationSource: &argoappv1.ApplicationSource{
					TargetRevision: invalidGitTaghash,
				},
				NoCache: true,
			},
			wantError: true,
			service:   newServiceWithCommitSHA(".", invalidGitTaghash),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifestResponse, err := tt.service.GenerateManifest(tt.ctx, tt.manifestRequest)
			if !tt.wantError {
				if err == nil {
					assert.Equal(t, manifestResponse.Revision, actualCommitSHA)
				} else {
					t.Errorf("unexpected error")
				}
			} else {
				if err == nil {
					t.Errorf("expected an error but did not throw one")
				}
			}

		})
	}

}

func TestHelmDependencyWithConcurrency(t *testing.T) {
	cleanup := func() {
		_ = os.Remove(filepath.Join("../../util/helm/testdata/helm2-dependency", helmDepUpMarkerFile))
		_ = os.RemoveAll(filepath.Join("../../util/helm/testdata/helm2-dependency", "charts"))
	}
	cleanup()
	defer cleanup()

	var wg sync.WaitGroup
	wg.Add(3)
	for i := 0; i < 3; i++ {
		go func() {
			res, err := helmTemplate("../../util/helm/testdata/helm2-dependency", "../..", nil, &apiclient.ManifestRequest{
				ApplicationSource: &argoappv1.ApplicationSource{},
			}, false)

			assert.NoError(t, err)
			assert.NotNil(t, res)
			wg.Done()
		}()
	}
	wg.Wait()
}
