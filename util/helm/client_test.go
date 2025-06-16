package helm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	utilio "github.com/argoproj/argo-cd/v3/util/io"
	"github.com/argoproj/argo-cd/v3/util/workloadidentity"
	"github.com/argoproj/argo-cd/v3/util/workloadidentity/mocks"
)

type fakeIndexCache struct {
	data []byte
}

type fakeTagsList struct {
	Tags []string `json:"tags"`
}

func (f *fakeIndexCache) SetHelmIndex(_ string, indexData []byte) error {
	f.data = indexData
	return nil
}

func (f *fakeIndexCache) GetHelmIndex(_ string, indexData *[]byte) error {
	*indexData = f.data
	return nil
}

func TestIndex(t *testing.T) {
	t.Run("Invalid", func(t *testing.T) {
		client := NewClient("", HelmCreds{}, false, "", "")
		_, err := client.GetIndex(false, 10000)
		require.Error(t, err)
	})
	t.Run("Stable", func(t *testing.T) {
		client := NewClient("https://argoproj.github.io/argo-helm", HelmCreds{}, false, "", "")
		index, err := client.GetIndex(false, 10000)
		require.NoError(t, err)
		assert.NotNil(t, index)
	})
	t.Run("BasicAuth", func(t *testing.T) {
		client := NewClient("https://argoproj.github.io/argo-helm", HelmCreds{
			Username: "my-password",
			Password: "my-username",
		}, false, "", "")
		index, err := client.GetIndex(false, 10000)
		require.NoError(t, err)
		assert.NotNil(t, index)
	})

	t.Run("Cached", func(t *testing.T) {
		fakeIndex := Index{Entries: map[string]Entries{"fake": {}}}
		data := bytes.Buffer{}
		err := yaml.NewEncoder(&data).Encode(fakeIndex)
		require.NoError(t, err)

		client := NewClient("https://argoproj.github.io/argo-helm", HelmCreds{}, false, "", "", WithIndexCache(&fakeIndexCache{data: data.Bytes()}))
		index, err := client.GetIndex(false, 10000)

		require.NoError(t, err)
		assert.Equal(t, fakeIndex, *index)
	})

	t.Run("Limited", func(t *testing.T) {
		client := NewClient("https://argoproj.github.io/argo-helm", HelmCreds{}, false, "", "")
		_, err := client.GetIndex(false, 100)

		assert.ErrorContains(t, err, "unexpected end of stream")
	})
}

func Test_nativeHelmChart_ExtractChart(t *testing.T) {
	client := NewClient("https://argoproj.github.io/argo-helm", HelmCreds{}, false, "", "")
	path, closer, err := client.ExtractChart("argo-cd", "0.7.1", false, math.MaxInt64, true)
	require.NoError(t, err)
	defer utilio.Close(closer)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func Test_nativeHelmChart_ExtractChartWithLimiter(t *testing.T) {
	client := NewClient("https://argoproj.github.io/argo-helm", HelmCreds{}, false, "", "")
	_, _, err := client.ExtractChart("argo-cd", "0.7.1", false, 100, false)
	require.Error(t, err, "error while iterating on tar reader: unexpected EOF")
}

func Test_nativeHelmChart_ExtractChart_insecure(t *testing.T) {
	client := NewClient("https://argoproj.github.io/argo-helm", HelmCreds{InsecureSkipVerify: true}, false, "", "")
	path, closer, err := client.ExtractChart("argo-cd", "0.7.1", false, math.MaxInt64, true)
	require.NoError(t, err)
	defer utilio.Close(closer)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func Test_normalizeChartName(t *testing.T) {
	t.Run("Test non-slashed name", func(t *testing.T) {
		n := normalizeChartName("mychart")
		assert.Equal(t, "mychart", n)
	})
	t.Run("Test single-slashed name", func(t *testing.T) {
		n := normalizeChartName("myorg/mychart")
		assert.Equal(t, "mychart", n)
	})
	t.Run("Test chart name with suborg", func(t *testing.T) {
		n := normalizeChartName("myorg/mysuborg/mychart")
		assert.Equal(t, "mychart", n)
	})
	t.Run("Test double-slashed name", func(t *testing.T) {
		n := normalizeChartName("myorg//mychart")
		assert.Equal(t, "mychart", n)
	})
	t.Run("Test invalid chart name - ends with slash", func(t *testing.T) {
		n := normalizeChartName("myorg/")
		assert.Equal(t, "myorg/", n)
	})
	t.Run("Test invalid chart name - is dot", func(t *testing.T) {
		n := normalizeChartName("myorg/.")
		assert.Equal(t, "myorg/.", n)
	})
	t.Run("Test invalid chart name - is two dots", func(t *testing.T) {
		n := normalizeChartName("myorg/..")
		assert.Equal(t, "myorg/..", n)
	})
}

func TestIsHelmOciRepo(t *testing.T) {
	assert.True(t, IsHelmOciRepo("demo.goharbor.io"))
	assert.True(t, IsHelmOciRepo("demo.goharbor.io:8080"))
	assert.False(t, IsHelmOciRepo("https://demo.goharbor.io"))
	assert.False(t, IsHelmOciRepo("https://demo.goharbor.io:8080"))
}

func TestGetIndexURL(t *testing.T) {
	urlTemplate := `https://gitlab.com/projects/%s/packages/helm/stable`
	t.Run("URL without escaped characters", func(t *testing.T) {
		rawURL := fmt.Sprintf(urlTemplate, "232323982")
		want := rawURL + "/index.yaml"
		got, err := getIndexURL(rawURL)
		assert.Equal(t, want, got)
		require.NoError(t, err)
	})
	t.Run("URL with escaped characters", func(t *testing.T) {
		rawURL := fmt.Sprintf(urlTemplate, "mygroup%2Fmyproject")
		want := rawURL + "/index.yaml"
		got, err := getIndexURL(rawURL)
		assert.Equal(t, want, got)
		require.NoError(t, err)
	})
	t.Run("URL with invalid escaped characters", func(t *testing.T) {
		rawURL := fmt.Sprintf(urlTemplate, "mygroup%**myproject")
		got, err := getIndexURL(rawURL)
		assert.Empty(t, got)
		require.Error(t, err)
	})
}

func TestGetTagsFromUrl(t *testing.T) {
	t.Run("should return tags correctly while following the link header", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Logf("called %s", r.URL.Path)
			var responseTags fakeTagsList
			w.Header().Set("Content-Type", "application/json")
			if !strings.Contains(r.URL.String(), "token") {
				w.Header().Set("Link", fmt.Sprintf("<https://%s%s?token=next-token>; rel=next", r.Host, r.URL.Path))
				responseTags = fakeTagsList{
					Tags: []string{"first"},
				}
			} else {
				responseTags = fakeTagsList{
					Tags: []string{
						"second",
						"2.8.0",
						"2.8.0-prerelease",
						"2.8.0_build",
						"2.8.0-prerelease_build",
						"2.8.0-prerelease.1_build.1234",
					},
				}
			}
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(responseTags))
		}))

		client := NewClient(server.URL, HelmCreds{InsecureSkipVerify: true}, true, "", "")

		tags, err := client.GetTags("mychart", true)
		require.NoError(t, err)
		assert.ElementsMatch(t, tags, []string{
			"first",
			"second",
			"2.8.0",
			"2.8.0-prerelease",
			"2.8.0+build",
			"2.8.0-prerelease+build",
			"2.8.0-prerelease.1+build.1234",
		})
	})

	t.Run("should return an error not when oci is not enabled", func(t *testing.T) {
		client := NewClient("example.com", HelmCreds{}, false, "", "")

		_, err := client.GetTags("my-chart", true)
		assert.ErrorIs(t, ErrOCINotEnabled, err)
	})
}

func TestGetTagsFromURLPrivateRepoAuthentication(t *testing.T) {
	username := "my-username"
	password := "my-password"
	expectedAuthorization := "Basic bXktdXNlcm5hbWU6bXktcGFzc3dvcmQ=" // base64(user:password)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("called %s", r.URL.Path)

		authorization := r.Header.Get("Authorization")

		if authorization == "" {
			w.Header().Set("WWW-Authenticate", `Basic realm="helm repo to get tags"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		assert.Equal(t, expectedAuthorization, authorization)

		responseTags := fakeTagsList{
			Tags: []string{
				"2.8.0",
				"2.8.0-prerelease",
				"2.8.0_build",
				"2.8.0-prerelease_build",
				"2.8.0-prerelease.1_build.1234",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		require.NoError(t, json.NewEncoder(w).Encode(responseTags))
	}))
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	testCases := []struct {
		name    string
		repoURL string
	}{
		{
			name:    "should login correctly when the repo path is in the server root with http scheme",
			repoURL: server.URL,
		},
		{
			name:    "should login correctly when the repo path is not in the server root with http scheme",
			repoURL: server.URL + "/my-repo",
		},
		{
			name:    "should login correctly when the repo path is in the server root without http scheme",
			repoURL: serverURL.Host,
		},
		{
			name:    "should login correctly when the repo path is not in the server root without http scheme",
			repoURL: serverURL.Host + "/my-repo",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			client := NewClient(testCase.repoURL, HelmCreds{
				InsecureSkipVerify: true,
				Username:           username,
				Password:           password,
			}, true, "", "")

			tags, err := client.GetTags("mychart", true)

			require.NoError(t, err)
			assert.ElementsMatch(t, tags, []string{
				"2.8.0",
				"2.8.0-prerelease",
				"2.8.0+build",
				"2.8.0-prerelease+build",
				"2.8.0-prerelease.1+build.1234",
			})
		})
	}
}

func TestGetTagsFromURLPrivateRepoWithAzureWorkloadIdentityAuthentication(t *testing.T) {
	expectedAuthorization := "Basic MDAwMDAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMDAwMDAwMDAwOmFjY2Vzc1Rva2Vu" // base64(00000000-0000-0000-0000-000000000000:accessToken)
	mockServerURL := ""
	mockedServerURL := func() string {
		return mockServerURL
	}

	workloadIdentityMock := new(mocks.TokenProvider)
	workloadIdentityMock.On("GetToken", "https://management.core.windows.net/.default").Return(&workloadidentity.Token{AccessToken: "accessToken"}, nil)

	mockServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("called %s", r.URL.Path)

		switch r.URL.Path {
		case "/v2/":
			w.Header().Set("Www-Authenticate", fmt.Sprintf(`Bearer realm=%q,service=%q`, mockedServerURL(), mockedServerURL()[8:]))
			w.WriteHeader(http.StatusUnauthorized)

		case "/oauth2/exchange":
			response := `{"refresh_token":"accessToken"}`
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(response))
			require.NoError(t, err)
		default:
			authorization := r.Header.Get("Authorization")

			if authorization == "" {
				w.Header().Set("WWW-Authenticate", `Basic realm="helm repo to get tags"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			assert.Equal(t, expectedAuthorization, authorization)

			responseTags := fakeTagsList{
				Tags: []string{
					"2.8.0",
					"2.8.0-prerelease",
					"2.8.0_build",
					"2.8.0-prerelease_build",
					"2.8.0-prerelease.1_build.1234",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(responseTags))
		}
	}))
	mockServerURL = mockServer.URL
	t.Cleanup(mockServer.Close)

	serverURL, err := url.Parse(mockServer.URL)
	require.NoError(t, err)

	testCases := []struct {
		name    string
		repoURL string
	}{
		{
			name:    "should login correctly when the repo path is in the server root with http scheme",
			repoURL: mockServer.URL,
		},
		{
			name:    "should login correctly when the repo path is not in the server root with http scheme",
			repoURL: mockServer.URL + "/my-repo",
		},
		{
			name:    "should login correctly when the repo path is in the server root without http scheme",
			repoURL: serverURL.Host,
		},
		{
			name:    "should login correctly when the repo path is not in the server root without http scheme",
			repoURL: serverURL.Host + "/my-repo",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			client := NewClient(testCase.repoURL, AzureWorkloadIdentityCreds{
				repoURL:            mockServer.URL[8:],
				InsecureSkipVerify: true,
				tokenProvider:      workloadIdentityMock,
			}, true, "", "")

			tags, err := client.GetTags("mychart", true)

			require.NoError(t, err)
			assert.ElementsMatch(t, tags, []string{
				"2.8.0",
				"2.8.0-prerelease",
				"2.8.0+build",
				"2.8.0-prerelease+build",
				"2.8.0-prerelease.1+build.1234",
			})
		})
	}
}

func TestGetTagsFromURLEnvironmentAuthentication(t *testing.T) {
	bearerToken := "Zm9vOmJhcg=="
	expectedAuthorization := "Basic " + bearerToken
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("called %s", r.URL.Path)

		authorization := r.Header.Get("Authorization")
		if authorization == "" {
			w.Header().Set("WWW-Authenticate", `Basic realm="helm repo to get tags"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		assert.Equal(t, expectedAuthorization, authorization)

		responseTags := fakeTagsList{
			Tags: []string{
				"2.8.0",
				"2.8.0-prerelease",
				"2.8.0_build",
				"2.8.0-prerelease_build",
				"2.8.0-prerelease.1_build.1234",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		require.NoError(t, json.NewEncoder(w).Encode(responseTags))
	}))
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	t.Setenv("DOCKER_CONFIG", tempDir)

	config := fmt.Sprintf(`{"auths":{%q:{"auth":%q}}}`, server.URL, bearerToken)
	require.NoError(t, os.WriteFile(configPath, []byte(config), 0o666))

	testCases := []struct {
		name    string
		repoURL string
	}{
		{
			name:    "should login correctly when the repo path is in the server root with http scheme",
			repoURL: server.URL,
		},
		{
			name:    "should login correctly when the repo path is not in the server root with http scheme",
			repoURL: server.URL + "/my-repo",
		},
		{
			name:    "should login correctly when the repo path is in the server root without http scheme",
			repoURL: serverURL.Host,
		},
		{
			name:    "should login correctly when the repo path is not in the server root without http scheme",
			repoURL: serverURL.Host + "/my-repo",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			client := NewClient(testCase.repoURL, HelmCreds{
				InsecureSkipVerify: true,
			}, true, "", "")

			tags, err := client.GetTags("mychart", true)

			require.NoError(t, err)
			assert.ElementsMatch(t, tags, []string{
				"2.8.0",
				"2.8.0-prerelease",
				"2.8.0+build",
				"2.8.0-prerelease+build",
				"2.8.0-prerelease.1+build.1234",
			})
		})
	}
}
