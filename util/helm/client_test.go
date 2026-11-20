package helm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	keysync "github.com/argoproj/pkg/v2/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
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
		_, err := client.GetIndex(t.Context(), false, 10000)
		require.Error(t, err)
	})
	t.Run("Stable", func(t *testing.T) {
		client := NewClient("https://argoproj.github.io/argo-helm", HelmCreds{}, false, "", "")
		index, err := client.GetIndex(t.Context(), false, 10000)
		require.NoError(t, err)
		assert.NotNil(t, index)
	})
	t.Run("BasicAuth", func(t *testing.T) {
		client := NewClient("https://argoproj.github.io/argo-helm", HelmCreds{
			Username: "my-password",
			Password: "my-username",
		}, false, "", "")
		index, err := client.GetIndex(t.Context(), false, 10000)
		require.NoError(t, err)
		assert.NotNil(t, index)
	})

	t.Run("Cached", func(t *testing.T) {
		fakeIndex := Index{Entries: map[string]Entries{"fake": {}}}
		data := bytes.Buffer{}
		err := yaml.NewEncoder(&data).Encode(fakeIndex)
		require.NoError(t, err)

		var requests atomic.Int32
		origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requests.Add(1)
			_, _ = w.Write(data.Bytes())
		}))
		t.Cleanup(origin.Close)
		client := NewClient(origin.URL, HelmCreds{}, false, "", "", WithIndexCache(&fakeIndexCache{}))
		_, err = client.GetIndex(t.Context(), true, 10000)
		require.NoError(t, err)
		index, err := client.GetIndex(t.Context(), false, 10000)

		require.NoError(t, err)
		assert.Equal(t, fakeIndex, *index)
		require.EqualValues(t, 1, requests.Load(), "a confirmed validator-less representation must remain warm")
	})

	t.Run("Limited", func(t *testing.T) {
		client := NewClient("https://argoproj.github.io/argo-helm", HelmCreds{}, false, "", "")
		_, err := client.GetIndex(t.Context(), false, 100)

		assert.ErrorContains(t, err, "unexpected end of stream")
	})
}

func Test_nativeHelmChart_ExtractChart(t *testing.T) {
	client := NewClient("https://argoproj.github.io/argo-helm", HelmCreds{}, false, "", "")
	path, closer, err := client.ExtractChart(t.Context(), "argo-cd", "0.7.1", false, math.MaxInt64, true)
	require.NoError(t, err)
	defer utilio.Close(closer)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func Test_nativeHelmChart_ExtractChartWithLimiter(t *testing.T) {
	client := NewClient("https://argoproj.github.io/argo-helm", HelmCreds{}, false, "", "")
	_, _, err := client.ExtractChart(t.Context(), "argo-cd", "0.7.1", false, 100, false)
	require.Error(t, err, "error while iterating on tar reader: unexpected EOF")
}

func Test_nativeHelmChart_ExtractChart_insecure(t *testing.T) {
	client := NewClient("https://argoproj.github.io/argo-helm", HelmCreds{InsecureSkipVerify: true}, false, "", "")
	path, closer, err := client.ExtractChart(t.Context(), "argo-cd", "0.7.1", false, math.MaxInt64, true)
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
	tests := []struct {
		name    string
		repoURL string
		want    bool
	}{
		{name: "hostname", repoURL: "demo.goharbor.io", want: true},
		{name: "hostname with port", repoURL: "demo.goharbor.io:8080", want: true},
		{name: "hostname with path", repoURL: "demo.goharbor.io/library", want: true},
		{name: "IPv4", repoURL: "192.168.100.150", want: true},
		{name: "IPv4 with port", repoURL: "192.168.100.150:1843", want: true},
		{name: "IPv4 with port and path", repoURL: "192.168.100.150:1843/library", want: true},
		{name: "IPv6 with port", repoURL: "[::1]:5000", want: true},
		{name: "empty", repoURL: "", want: false},
		{name: "https hostname", repoURL: "https://demo.goharbor.io", want: false},
		{name: "https hostname with port", repoURL: "https://demo.goharbor.io:8080", want: false},
		{name: "https hostname with path", repoURL: "https://demo.goharbor.io/library", want: false},
		{name: "https IPv4 with port", repoURL: "https://192.168.100.150:1843", want: false},
		{name: "oci IPv4 with port", repoURL: "oci://192.168.100.150:1843", want: false},
		// Unlikely inputs in practice; they document the boundary of what the check rejects.
		{name: "non-numeric port", repoURL: "demo.goharbor.io:abc", want: false},
		{name: "whitespace", repoURL: "demo.goharbor.io ", want: false},
		{name: "fragment", repoURL: "demo.goharbor.io#latest", want: false},
		{name: "path without host", repoURL: "/192.168.100.150:1843", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsHelmOciRepo(tt.repoURL))
		})
	}
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

		tags, err := client.GetTags(t.Context(), "mychart", true)
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

		_, err := client.GetTags(t.Context(), "my-chart", true)
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

			tags, err := client.GetTags(t.Context(), "mychart", true)

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

	workloadIdentityMock := &mocks.TokenProvider{}
	workloadIdentityMock.EXPECT().GetToken("https://management.core.windows.net/.default").Return(&workloadidentity.Token{AccessToken: "accessToken"}, nil)

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

			tags, err := client.GetTags(t.Context(), "mychart", true)

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

			tags, err := client.GetTags(t.Context(), "mychart", true)

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

func TestGetTagsCaching(t *testing.T) {
	requestCount := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		t.Logf("request %d called %s", requestCount, r.URL.Path)

		responseTags := fakeTagsList{
			Tags: []string{
				"1.0.0",
				"1.1.0",
				"2.0.0_beta",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		require.NoError(t, json.NewEncoder(w).Encode(responseTags))
	}))
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	t.Run("should cache tags correctly", func(t *testing.T) {
		cache := &fakeIndexCache{}
		client := NewClient(serverURL.Host, HelmCreds{
			InsecureSkipVerify: true,
		}, true, "", "", WithIndexCache(cache))

		tags1, err := client.GetTags(t.Context(), "mychart", false)
		require.NoError(t, err)
		assert.ElementsMatch(t, tags1, []string{
			"1.0.0",
			"1.1.0",
			"2.0.0+beta",
		})
		assert.Equal(t, 1, requestCount)

		requestCount = 0

		tags2, err := client.GetTags(t.Context(), "mychart", false)
		require.NoError(t, err)
		assert.ElementsMatch(t, tags2, []string{
			"1.0.0",
			"1.1.0",
			"2.0.0+beta",
		})
		assert.Equal(t, 0, requestCount)

		assert.NotEmpty(t, cache.data)

		type entriesStruct struct {
			Tags []string
		}
		var entries entriesStruct
		err = json.Unmarshal(cache.data, &entries)
		require.NoError(t, err)
		assert.ElementsMatch(t, entries.Tags, []string{
			"1.0.0",
			"1.1.0",
			"2.0.0+beta",
		})
	})

	t.Run("should bypass cache when noCache is true", func(t *testing.T) {
		cache := &fakeIndexCache{}
		client := NewClient(serverURL.Host, HelmCreds{
			InsecureSkipVerify: true,
		}, true, "", "", WithIndexCache(cache))

		requestCount = 0

		tags1, err := client.GetTags(t.Context(), "mychart", true)
		require.NoError(t, err)
		assert.ElementsMatch(t, tags1, []string{
			"1.0.0",
			"1.1.0",
			"2.0.0+beta",
		})
		assert.Equal(t, 1, requestCount)

		tags2, err := client.GetTags(t.Context(), "mychart", true)
		require.NoError(t, err)
		assert.ElementsMatch(t, tags2, []string{
			"1.0.0",
			"1.1.0",
			"2.0.0+beta",
		})
		assert.Equal(t, 2, requestCount)
	})
}

func TestGetTagsUsesHTTP2(t *testing.T) {
	t.Run("should negotiate HTTP/2 when TLS is configured", func(t *testing.T) {
		var requestProtos []string
		server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestProtos = append(requestProtos, r.Proto)
			t.Logf("called %s with proto %s", r.URL.Path, r.Proto)

			responseTags := fakeTagsList{
				Tags: []string{"1.0.0"},
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(responseTags))
		}))
		// httptest.NewTLSServer only advertises http/1.1 in ALPN, so we must
		// configure the server to also offer h2 for HTTP/2 negotiation to work.
		server.TLS = &tls.Config{NextProtos: []string{"h2", "http/1.1"}}
		server.StartTLS()
		t.Cleanup(server.Close)

		client := NewClient(server.URL, HelmCreds{InsecureSkipVerify: true}, true, "", "")

		tags, err := client.GetTags(t.Context(), "mychart", true)
		require.NoError(t, err)
		assert.Equal(t, []string{"1.0.0"}, tags)

		// Verify that at least one request used HTTP/2. When ForceAttemptHTTP2 is
		// not set on the Transport, Go's TLS stack won't negotiate h2 even though
		// the server supports it, because a custom TLSClientConfig disables the
		// automatic HTTP/2 setup.
		require.NotEmpty(t, requestProtos, "expected at least one request to the server")
		hasHTTP2 := slices.Contains(requestProtos, "HTTP/2.0")
		assert.True(t, hasHTTP2, "expected at least one HTTP/2 request, but got protocols: %v", requestProtos)
	})
}

func TestLoadRepoIndexUsesHTTP2(t *testing.T) {
	t.Run("should negotiate HTTP/2 when fetching index", func(t *testing.T) {
		var requestProto string
		server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestProto = r.Proto
			t.Logf("called %s with proto %s", r.URL.Path, r.Proto)

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`apiVersion: v1
entries: {}
`))
		}))
		server.TLS = &tls.Config{NextProtos: []string{"h2", "http/1.1"}}
		server.StartTLS()
		t.Cleanup(server.Close)

		client := NewClient(server.URL, HelmCreds{InsecureSkipVerify: true}, false, "", "")

		_, err := client.GetIndex(t.Context(), false, 10000)
		require.NoError(t, err)

		assert.Equal(t, "HTTP/2.0", requestProto, "expected HTTP/2 request for index fetch, but got %s", requestProto)
	})
}

func TestUserAgentIsSet(t *testing.T) {
	t.Run("Default User-Agent for traditional Helm repo", func(t *testing.T) {
		// Create a test server that captures the User-Agent header
		receivedUserAgent := ""
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedUserAgent = r.Header.Get("User-Agent")

			// Return a valid minimal index.yaml
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`apiVersion: v1
entries: {}
`))
		}))
		defer ts.Close()

		// Create client and make a request
		client := NewClient(ts.URL, HelmCreds{}, false, "", "")
		_, err := client.GetIndex(t.Context(), false, 10000)
		require.NoError(t, err)

		// Verify User-Agent was set and contains expected components
		assert.NotEmpty(t, receivedUserAgent, "User-Agent header should be set")
		assert.Contains(t, receivedUserAgent, "argocd-repo-server", "User-Agent should contain 'argocd-repo-server'")
		t.Logf("User-Agent sent: %s", receivedUserAgent)
	})

	t.Run("Custom User-Agent via WithUserAgent option", func(t *testing.T) {
		// Create a test server that captures the User-Agent header
		receivedUserAgent := ""
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedUserAgent = r.Header.Get("User-Agent")

			// Return a valid minimal index.yaml
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`apiVersion: v1
entries: {}
`))
		}))
		defer ts.Close()

		// Create client with custom User-Agent
		customUA := "my-custom-app/1.2.3 (contact@example.com)"
		client := NewClient(ts.URL, HelmCreds{}, false, "", "", WithUserAgent(customUA))
		_, err := client.GetIndex(t.Context(), false, 10000)
		require.NoError(t, err)

		// Verify custom User-Agent was used
		assert.Equal(t, customUA, receivedUserAgent, "Custom User-Agent should be used")
		t.Logf("Custom User-Agent sent: %s", receivedUserAgent)
	})
}

func TestUserAgentRequiredByServer(t *testing.T) {
	t.Run("Server rejects requests without User-Agent", func(t *testing.T) {
		// Create a test server that mimics Wikimedia's behavior
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userAgent := r.Header.Get("User-Agent")

			t.Logf("Server received User-Agent: '%s'", userAgent)

			if userAgent == "" {
				// Mimic Wikimedia's rejection of empty User-Agent
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`authorization failed: Please set a user-agent and respect our robot policy`))
				return
			}

			// Accept request with User-Agent
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`apiVersion: v1
entries: {}
`))
		}))
		defer ts.Close()

		// Create client (should automatically set User-Agent)
		client := NewClient(ts.URL, HelmCreds{}, false, "", "")
		_, err := client.GetIndex(t.Context(), false, 10000)

		// Should succeed because our implementation sets User-Agent
		require.NoError(t, err, "Request should succeed with User-Agent set")
		t.Log("Success! Server accepted request with User-Agent")
	})
}

func TestUserAgentPriority(t *testing.T) {
	t.Run("Custom User-Agent set via WithUserAgent option", func(t *testing.T) {
		// Create a test server that captures the User-Agent header
		receivedUserAgent := ""
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedUserAgent = r.Header.Get("User-Agent")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`apiVersion: v1
entries: {}
`))
		}))
		defer ts.Close()

		// Set custom User-Agent using WithUserAgent option
		customUA := "CustomAgent/1.0 (test)"
		client := NewClient(ts.URL, HelmCreds{}, false, "", "", WithUserAgent(customUA))
		_, err := client.GetIndex(t.Context(), false, 10000)
		require.NoError(t, err)

		// Verify custom User-Agent was used
		assert.Equal(t, customUA, receivedUserAgent, "Custom User-Agent should be used when set")
		t.Logf("Custom User-Agent sent: %s", receivedUserAgent)
	})

	t.Run("Default User-Agent used when no custom value provided", func(t *testing.T) {
		// Create a test server that captures the User-Agent header
		receivedUserAgent := ""
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedUserAgent = r.Header.Get("User-Agent")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`apiVersion: v1
entries: {}
`))
		}))
		defer ts.Close()

		// Create client without custom User-Agent
		client := NewClient(ts.URL, HelmCreds{}, false, "", "")
		_, err := client.GetIndex(t.Context(), false, 10000)
		require.NoError(t, err)

		// Verify default User-Agent was used
		assert.Contains(t, receivedUserAgent, "argocd-repo-server", "Should use default User-Agent format")
		t.Logf("Default User-Agent sent: %s", receivedUserAgent)
	})
}

func TestWithPlainHTTP(t *testing.T) {
	t.Run("not set by default", func(t *testing.T) {
		c := NewClient("my.registry.com", HelmCreds{}, true, "", "")
		assert.False(t, c.(*nativeHelmChart).plainHTTP)
	})

	t.Run("set via WithPlainHTTP option", func(t *testing.T) {
		c := NewClient("my.registry.com", HelmCreds{}, true, "", "", WithPlainHTTP())
		assert.True(t, c.(*nativeHelmChart).plainHTTP)
	})
}

func Test_nativeHelmChart_ExtractChartRespectsChartCacheExpiration(t *testing.T) {
	const (
		chartName    = "test-chart"
		chartVersion = "1.0.0"
	)

	var (
		mu            sync.Mutex
		chartRequests int
		chartArchive  = testHelmChartArchive(t, chartName, chartVersion, "foo: first\n")
		server        *httptest.Server
	)

	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.yaml":
			w.Header().Set("Content-Type", "application/x-yaml")
			_, _ = fmt.Fprintf(w, `apiVersion: v1
entries:
  %[1]s:
  - apiVersion: v2
    name: %[1]s
    version: %[2]s
    urls:
    - %[3]s/%[1]s-%[2]s.tgz
`, chartName, chartVersion, server.URL)
		case "/" + chartName + "-" + chartVersion + ".tgz":
			mu.Lock()
			chartRequests++
			body := append([]byte(nil), chartArchive...)
			mu.Unlock()

			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(body)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, HelmCreds{}, false, "", "",
		WithChartPaths(utilio.NewRandomizedTempPaths(t.TempDir())),
		WithHelmChartCacheExpiration(time.Hour),
	)
	nativeClient, ok := client.(*nativeHelmChart)
	require.True(t, ok)

	extractValues := func() string {
		t.Helper()
		path, closer, err := client.ExtractChart(t.Context(), chartName, chartVersion, false, math.MaxInt64, false)
		require.NoError(t, err)
		defer utilio.Close(closer)

		values, err := os.ReadFile(filepath.Join(path, "values.yaml"))
		require.NoError(t, err)
		return string(values)
	}
	requireChartRequests := func(expected int) {
		t.Helper()
		mu.Lock()
		defer mu.Unlock()
		require.Equal(t, expected, chartRequests)
	}

	require.Equal(t, "foo: first\n", extractValues())
	requireChartRequests(1)

	secondChartArchive := testHelmChartArchive(t, chartName, chartVersion, "foo: second\n")
	mu.Lock()
	chartArchive = secondChartArchive
	mu.Unlock()

	require.Equal(t, "foo: first\n", extractValues())
	requireChartRequests(1)

	cachedChartPath, err := nativeClient.getCachedChartPath(chartName, chartVersion)
	require.NoError(t, err)
	expired := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(cachedChartPath, expired, expired))

	require.Equal(t, "foo: second\n", extractValues())
	requireChartRequests(2)
}

// observingKeyLock wraps a sync.KeyLock so that tests can observe when a key lock is acquired
// or merely attempted, without polling or sleeping.
type observingKeyLock struct {
	keysync.KeyLock
	beforeLock func(key string)
	afterLock  func(key string)
}

func (l observingKeyLock) Lock(key string) {
	if l.beforeLock != nil {
		l.beforeLock(key)
	}
	l.KeyLock.Lock(key)
	if l.afterLock != nil {
		l.afterLock(key)
	}
}

func Test_nativeHelmChart_CleanChartCacheWaitsForExtraction(t *testing.T) {
	const (
		chartName    = "test-chart"
		chartVersion = "1.0.0"
		repoURL      = "https://example.com/charts"
	)

	// A single chart cache root is shared by every Helm client of a repo-server instance, so both
	// clients below resolve the same cached archive for the same chart and version.
	chartPaths := utilio.NewRandomizedTempPaths(t.TempDir())
	extractionLocked := make(chan string, 1)
	cleanupLockAttempt := make(chan string, 1)

	// Both clients wrap the process-wide lock, which is what helm.NewClient hands to every
	// repo-server request.
	extractor := NewClientWithLock(repoURL, HelmCreds{}, observingKeyLock{
		KeyLock:   globalLock,
		afterLock: func(key string) { extractionLocked <- key },
	}, false, "", "", WithChartPaths(chartPaths))
	cleaner := NewClientWithLock(repoURL, HelmCreds{}, observingKeyLock{
		KeyLock:    globalLock,
		beforeLock: func(key string) { cleanupLockAttempt <- key },
	}, false, "", "", WithChartPaths(chartPaths))

	nativeExtractor, ok := extractor.(*nativeHelmChart)
	require.True(t, ok)
	nativeCleaner, ok := cleaner.(*nativeHelmChart)
	require.True(t, ok)

	cachePath, err := nativeCleaner.getCachedChartPath(chartName, chartVersion)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(cachePath), 0o755))
	require.NoError(t, os.WriteFile(cachePath, []byte("cached chart archive"), 0o600))

	// Simulate an in-flight ExtractChart, which holds the key lock while untarChart reads and
	// extracts the cached archive.
	nativeExtractor.repoLock.Lock(cachePath)
	require.Equal(t, cachePath, <-extractionLocked)

	cleaned := make(chan error, 1)
	go func() { cleaned <- cleaner.CleanChartCache(chartName, chartVersion) }()
	select {
	case key := <-cleanupLockAttempt:
		require.Equal(t, cachePath, key)
		require.FileExists(t, cachePath)
	case err := <-cleaned:
		require.NoError(t, err)
		t.Fatal("chart cache cleanup completed while extraction still held the archive lock")
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for chart cache cleanup to attempt the archive lock")
	}

	nativeExtractor.repoLock.Unlock(cachePath)

	select {
	case err := <-cleaned:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for chart cache cleanup to complete")
	}
	require.NoFileExists(t, cachePath)
}

func Test_NewClient_SharesLockBetweenClients(t *testing.T) {
	// The repo-server creates one Helm client per request. They must all share the process-wide
	// lock, otherwise cache cleanup cannot serialize with chart extraction across requests.
	first, ok := NewClient("https://example.com/charts", HelmCreds{}, false, "", "").(*nativeHelmChart)
	require.True(t, ok)
	second, ok := NewClient("https://example.com/charts", HelmCreds{}, false, "", "").(*nativeHelmChart)
	require.True(t, ok)

	require.Equal(t, globalLock, first.repoLock)
	require.Equal(t, first.repoLock, second.repoLock)
}

func testHelmChartArchive(t *testing.T, chartName, chartVersion, values string) []byte {
	t.Helper()

	var archive bytes.Buffer
	gzipWriter := gzip.NewWriter(&archive)
	tarWriter := tar.NewWriter(gzipWriter)

	writeFile := func(name, contents string) {
		t.Helper()
		header := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(contents)),
		}
		require.NoError(t, tarWriter.WriteHeader(header))
		_, err := tarWriter.Write([]byte(contents))
		require.NoError(t, err)
	}

	writeFile(filepath.ToSlash(filepath.Join(chartName, "Chart.yaml")), fmt.Sprintf(`apiVersion: v2
name: %s
version: %s
`, chartName, chartVersion))
	writeFile(filepath.ToSlash(filepath.Join(chartName, "values.yaml")), values)

	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzipWriter.Close())
	return archive.Bytes()
}

func TestValidEntityTag(t *testing.T) {
	t.Run("Strong", func(t *testing.T) {
		assert.True(t, validEntityTag(`"abc"`))
	})
	t.Run("Weak", func(t *testing.T) {
		assert.True(t, validEntityTag(`W/"abc"`))
	})
	t.Run("Empty", func(t *testing.T) {
		assert.False(t, validEntityTag(""))
	})
	t.Run("Unquoted", func(t *testing.T) {
		assert.False(t, validEntityTag("abc"))
	})
	t.Run("Wildcard", func(t *testing.T) {
		assert.False(t, validEntityTag("*"))
	})
	t.Run("InternalQuote", func(t *testing.T) {
		assert.False(t, validEntityTag(`"ab"c"`))
	})
	t.Run("ControlChars", func(t *testing.T) {
		assert.False(t, validEntityTag("\"a\x01b\""))
	})
	t.Run("Space", func(t *testing.T) {
		assert.False(t, validEntityTag(`"a b"`))
	})
	t.Run("MissingTrailingQuote", func(t *testing.T) {
		assert.False(t, validEntityTag(`"abc`))
	})
}

func TestMatchingNotModifiedETag(t *testing.T) {
	t.Run("OmittedETagInResponse", func(t *testing.T) {
		assert.True(t, matchingNotModifiedETag(`"abc"`, ""))
	})
	t.Run("StrongMatch", func(t *testing.T) {
		assert.True(t, matchingNotModifiedETag(`"abc"`, `"abc"`))
	})
	t.Run("StrongMismatch", func(t *testing.T) {
		assert.False(t, matchingNotModifiedETag(`"abc"`, `"abd"`))
	})
	t.Run("WeakMatch", func(t *testing.T) {
		assert.True(t, matchingNotModifiedETag(`W/"abc"`, `W/"abc"`))
		assert.True(t, matchingNotModifiedETag(`"abc"`, `W/"abc"`))
	})
	t.Run("WeakCachedCannotBeValidatedByStrong", func(t *testing.T) {
		assert.False(t, matchingNotModifiedETag(`W/"abc"`, `"abc"`))
	})
	t.Run("MalformedResponseTag", func(t *testing.T) {
		assert.False(t, matchingNotModifiedETag(`"abc"`, "*"))
		assert.False(t, matchingNotModifiedETag(`"abc"`, "abc"))
	})
}

func TestApplyConditionalHeaders(t *testing.T) {
	t.Run("ETagTakesPrecedence", func(t *testing.T) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/index.yaml", http.NoBody)
		require.NoError(t, err)
		applyConditionalHeaders(req, `"etag"`, "Mon, 02 Jan 2006 15:04:05 GMT")
		assert.Equal(t, `"etag"`, req.Header.Get("If-None-Match"))
		assert.Empty(t, req.Header.Get("If-Modified-Since"))
	})
	t.Run("FallbackToLastModified", func(t *testing.T) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/index.yaml", http.NoBody)
		require.NoError(t, err)
		applyConditionalHeaders(req, "", "Mon, 02 Jan 2006 15:04:05 GMT")
		assert.Empty(t, req.Header.Get("If-None-Match"))
		assert.Equal(t, "Mon, 02 Jan 2006 15:04:05 GMT", req.Header.Get("If-Modified-Since"))
	})
	t.Run("InvalidValidatorsNotSent", func(t *testing.T) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/index.yaml", http.NoBody)
		require.NoError(t, err)
		applyConditionalHeaders(req, "*", "not-a-date")
		assert.Empty(t, req.Header.Get("If-None-Match"))
		assert.Empty(t, req.Header.Get("If-Modified-Since"))
	})
}

func TestSingleResponseValidator(t *testing.T) {
	header := http.Header{}
	header.Set("ETag", `"abc"`)
	assert.Equal(t, `"abc"`, singleResponseValidator(header, "ETag"))

	header.Add("ETag", `"a"`)
	header.Add("ETag", `"b"`)
	assert.Equal(t, "invalid-multiple-ETag", singleResponseValidator(header, "ETag"))

	// Map-style assignment under a canonically-cased key must also be
	// detected: Go only exposes it through the canonical key.
	header = http.Header{}
	header[http.CanonicalHeaderKey("ETag")] = []string{`"a"`, `"b"`}
	assert.Equal(t, "invalid-multiple-ETag", singleResponseValidator(header, "ETag"))

	header = http.Header{}
	header[http.CanonicalHeaderKey("ETag")] = []string{`"a"`}
	assert.Equal(t, `"a"`, singleResponseValidator(header, "ETag"))
}

func TestSupportsIndexVary(t *testing.T) {
	t.Run("EmptyHeader", func(t *testing.T) {
		client := &nativeHelmChart{}
		assert.True(t, client.supportsIndexVary(http.Header{}))
	})
	t.Run("AllowlistedFields", func(t *testing.T) {
		client := &nativeHelmChart{}
		header := http.Header{}
		header.Set("Vary", "Accept-Encoding, User-Agent")
		assert.True(t, client.supportsIndexVary(header))
	})
	t.Run("UnknownField", func(t *testing.T) {
		client := &nativeHelmChart{}
		header := http.Header{}
		header.Set("Vary", "X-Custom-Header")
		assert.False(t, client.supportsIndexVary(header))
	})
	t.Run("AuthorizationAnonymousIsUnsafe", func(t *testing.T) {
		client := &nativeHelmChart{}
		header := http.Header{}
		header.Set("Vary", "Authorization")
		assert.False(t, client.supportsIndexVary(header))
	})
	t.Run("AuthorizationScopedIsSafe", func(t *testing.T) {
		client := &nativeHelmChart{scopedByCreds: true}
		header := http.Header{}
		header.Set("Vary", "Authorization")
		assert.True(t, client.supportsIndexVary(header))
	})
}

func TestHelmIndexRecordFresh(t *testing.T) {
	t.Run("FreshRecord", func(t *testing.T) {
		record := &HelmIndexRecord{StoredAt: time.Now().UTC().Format(time.RFC3339Nano)}
		assert.True(t, record.Fresh(time.Minute))
	})
	t.Run("StaleRecord", func(t *testing.T) {
		record := &HelmIndexRecord{StoredAt: time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339Nano)}
		assert.False(t, record.Fresh(time.Minute))
	})
	t.Run("ZeroFreshnessNeverFresh", func(t *testing.T) {
		record := &HelmIndexRecord{StoredAt: time.Now().UTC().Format(time.RFC3339Nano)}
		assert.False(t, record.Fresh(0))
	})
	t.Run("MissingTimestamp", func(t *testing.T) {
		record := &HelmIndexRecord{}
		assert.False(t, record.Fresh(time.Minute))
	})
	t.Run("FutureTimestampIsNotFresh", func(t *testing.T) {
		record := &HelmIndexRecord{StoredAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)}
		assert.False(t, record.Fresh(time.Minute))
	})
}

func TestIndexURLIdentityBindsUserAgentAndURL(t *testing.T) {
	client := &nativeHelmChart{}
	identityA, err := client.indexURLIdentityContext(t.Context(), "https://example.com/index.yaml")
	require.NoError(t, err)
	identityB, err := client.indexURLIdentityContext(t.Context(), "https://example.com/index.yaml")
	require.NoError(t, err)
	assert.Equal(t, identityA, identityB)

	other, err := client.indexURLIdentityContext(t.Context(), "https://example.org/index.yaml")
	require.NoError(t, err)
	assert.NotEqual(t, identityA, other)

	// Identity must not contain plaintext secrets from URL userinfo.
	withCreds, err := (&nativeHelmChart{}).indexURLIdentityContext(t.Context(), "https://user:secret@example.com/index.yaml")
	require.NoError(t, err)
	assert.NotContains(t, withCreds, "secret")
}

func TestMatchesIndexRequest(t *testing.T) {
	client := &nativeHelmChart{}
	identity, err := client.indexURLIdentityContext(t.Context(), "https://example.com/index.yaml")
	require.NoError(t, err)
	assert.True(t, client.matchesIndexRequest(t.Context(), identity))
	assert.False(t, client.matchesIndexRequest(t.Context(), "bogus"))
}

func TestResponseValidatorHeaders(t *testing.T) {
	header := http.Header{}
	header.Set("ETag", `"v1"`)
	header.Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
	etag, modified := responseValidatorHeaders(header)
	assert.Equal(t, `"v1"`, etag)
	assert.Equal(t, "Mon, 02 Jan 2006 15:04:05 GMT", modified)

	etag, modified = responseValidatorHeaders(nil)
	assert.Empty(t, etag)
	assert.Empty(t, modified)
}

func TestInMemoryIndexValidatorsBodyBinding(t *testing.T) {
	store := newInMemoryIndexValidators()
	bodyA := []byte("index-a")
	bodyB := []byte("index-b")

	store.setForResponse("repo", bodyA, `"etag-a"`, "Mon, 02 Jan 2006 15:04:05 GMT", "identity-a")

	etag, modified, identity := store.getForResponse("repo", bodyA)
	assert.Equal(t, `"etag-a"`, etag)
	assert.Equal(t, "Mon, 02 Jan 2006 15:04:05 GMT", modified)
	assert.Equal(t, "identity-a", identity)

	// A different body must not reuse validators bound to bodyA.
	etag, _, _ = store.getForResponse("repo", bodyB)
	assert.Empty(t, etag)

	// A different repo must not reuse validators bound to "repo".
	etag, _, _ = store.getForResponse("other", bodyA)
	assert.Empty(t, etag)

	// A validator that was never bound (e.g. restored from an external
	// writer) must not be eligible for a 304 against different bytes.
	require.NoError(t, store.SetHelmIndexValidators("repo", `"etag-b"`, ""))
	etag, _, _ = store.getForResponse("repo", bodyA)
	assert.Empty(t, etag, "unbound validators must not match a body")

	// Setting both validators empty removes the entry entirely.
	store.setForResponse("repo", bodyA, `"etag-a"`, "", "identity-a")
	require.NoError(t, store.SetHelmIndexValidators("repo", "", ""))
	_, _, identity = store.getForResponse("repo", bodyA)
	assert.Empty(t, identity)
}

func TestDefaultIndexCacheExpirationValue(t *testing.T) {
	assert.Equal(t, time.Minute, DefaultIndexCacheExpiration)
}

type fakeIndex struct {
	APIVersion string                 `yaml:"apiVersion"`
	Entries    map[string]fakeEntries `yaml:"entries"`
}

type fakeEntry struct {
	Version string `yaml:"version"`
}

type fakeEntries []fakeEntry

// newIndexBody emits a realistic index.yaml in the shape the pinned Helm
// client produces: apiVersion v1, lowercased keys, string versions.
func newIndexBody(version string) []byte {
	index := fakeIndex{APIVersion: "v1", Entries: map[string]fakeEntries{"fake": []fakeEntry{{Version: version}}}}
	data, err := yaml.Marshal(index)
	if err != nil {
		panic(err)
	}
	return data
}

type recordingIndexCache struct {
	data []byte
	sets atomic.Int64
	gets atomic.Int64
}

func (f *recordingIndexCache) SetHelmIndex(_ string, indexData []byte) error {
	f.sets.Add(1)
	f.data = indexData
	return nil
}

func (f *recordingIndexCache) GetHelmIndex(_ string, indexData *[]byte) error {
	f.gets.Add(1)
	*indexData = f.data
	return nil
}

// recordIndexCache mimics the repo-server record contract: one atomic record
// carrying body + validators + a freshness timestamp. Freshness follows the
// same semantics as reposerver/cache: a record is fresh while now-StoredAt
// stays inside the expiration window.
type recordIndexCache struct {
	mu         sync.Mutex
	record     *HelmIndexRecord
	expiration time.Duration // <=0: always stale
	recordSets atomic.Int64
	recordGets atomic.Int64
}

func (f *recordIndexCache) SetHelmIndexRecord(_ string, indexData []byte, etag string, lastModified string) error {
	f.recordSets.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	digest := sha256.Sum256(indexData)
	f.record = &HelmIndexRecord{Body: bytes.Clone(indexData), BodyDigest: hex.EncodeToString(digest[:]), ETag: etag, LastModified: lastModified, StoredAt: time.Now().UTC().Format(time.RFC3339Nano)}
	return nil
}

func (f *recordIndexCache) SetHelmIndexRetainedRecord(_ string, record *HelmIndexRecord) error {
	f.recordSets.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	clone := *record
	if record.BodyDigest == "" {
		digest := sha256.Sum256(record.Body)
		clone.BodyDigest = hex.EncodeToString(digest[:])
	}
	f.record = &clone
	return nil
}

func (f *recordIndexCache) GetHelmIndexRetainedRecord(_ string) (*HelmIndexRecord, error) {
	f.recordGets.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.record == nil {
		return nil, cacheutil.ErrCacheMiss
	}
	if !f.record.Fresh(f.expiration) {
		// Return the record anyway: the caller revalidates stale records with
		// the origin instead of discarding them.
		return f.record, nil
	}
	return f.record, nil
}

func (f *recordIndexCache) SetHelmIndex(_ string, indexData []byte) error {
	f.recordSets.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record = &HelmIndexRecord{Body: bytes.Clone(indexData), StoredAt: time.Now().UTC().Format(time.RFC3339Nano)}
	return nil
}

func (f *recordIndexCache) GetHelmIndex(_ string, indexData *[]byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	// Mirror reposerver semantics: a raw-body read only succeeds for a FRESH
	// record. A stale record must not be served without revalidation, so the
	// raw view misses and the client falls back to the conditional GET path.
	if f.record == nil || !f.record.Fresh(f.expiration) {
		*indexData = nil
		return cacheutil.ErrCacheMiss
	}
	*indexData = f.record.Body
	return nil
}

func TestIndexConditionalRevalidation(t *testing.T) {
	originRequests := atomic.Int64{}
	etag := `"index-etag-v1"`
	body := newIndexBody("1.0.0")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originRequests.Add(1)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Content-Type", "text/yaml")
		_, _ = w.Write(body)
	}))

	t.Run("SecondFetchIsConditionalAndReusesCachedBody", func(t *testing.T) {
		// A record-backed cache with a STALE record forces the client to
		// revalidate with the origin instead of trusting the cached bytes.
		cacheHeld := &recordIndexCache{expiration: time.Minute}
		client := NewClient(server.URL, HelmCreds{InsecureSkipVerify: true}, false, "", "", WithIndexCache(cacheHeld))

		index, err := client.GetIndex(t.Context(), false, 1024*1024)
		require.NoError(t, err)
		require.NotEmpty(t, index.Entries["fake"], "index must decode with entries")
		assert.Equal(t, "1.0.0", index.Entries["fake"][0].Version)
		firstCount := originRequests.Load()
		require.Equal(t, int64(1), firstCount)
		require.Equal(t, int64(1), cacheHeld.recordSets.Load(), "first fetch must publish the record")

		// Age the record past the freshness window so the next fetch must
		// revalidate. Failing-before: the client either re-downloads the
		// whole index (no conditional GET) or trusts stale bytes outright.
		cacheHeld.mu.Lock()
		stale := *cacheHeld.record
		stale.StoredAt = time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339Nano)
		cacheHeld.record = &stale
		cacheHeld.mu.Unlock()

		index2, err := client.GetIndex(t.Context(), false, 1024*1024)
		require.NoError(t, err)
		assert.Equal(t, "1.0.0", index2.Entries["fake"][0].Version)
		assert.Equal(t, firstCount+1, originRequests.Load(), "second fetch must revalidate with exactly one conditional GET")
		assert.Equal(t, int64(1), cacheHeld.recordSets.Load(), "a 304 must not republish a new record generation")
	})

	t.Run("ChangedIndexIsRefetched", func(t *testing.T) {
		newBody := newIndexBody("2.0.0")
		server2 := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("If-None-Match") == etag {
				// Simulate the origin having a newer representation than the
				// cached one: a 200 with a new ETag, not a 304.
				w.Header().Set("ETag", `"index-etag-v2"`)
				_, _ = w.Write(newBody)
				return
			}
			w.Header().Set("ETag", etag)
			_, _ = w.Write(body)
		}))
		cacheHeld := &recordIndexCache{expiration: time.Minute}
		client := NewClient(server2.URL, HelmCreds{InsecureSkipVerify: true}, false, "", "", WithIndexCache(cacheHeld))

		index, err := client.GetIndex(t.Context(), false, 1024*1024)
		require.NoError(t, err)
		assert.Equal(t, "1.0.0", index.Entries["fake"][0].Version)

		cacheHeld.mu.Lock()
		stale := *cacheHeld.record
		stale.StoredAt = time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339Nano)
		cacheHeld.record = &stale
		cacheHeld.mu.Unlock()

		index2, err := client.GetIndex(t.Context(), false, 1024*1024)
		require.NoError(t, err)
		assert.Equal(t, "2.0.0", index2.Entries["fake"][0].Version)
		assert.Equal(t, int64(2), cacheHeld.recordSets.Load(), "changed index must republish")
	})
}

func TestIndexCredentialScopeIsolation(t *testing.T) {
	const user = "secret-user"
	const pass = "secret-pass"
	body := newIndexBody("1.0.0")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, _ := r.BasicAuth()
		if username != "" {
			if username != user || password != pass {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		w.Header().Set("ETag", `"scoped-etag"`)
		_, _ = w.Write(body)
	}))

	privateCache := &recordingIndexCache{}
	anonClient := NewClient(server.URL, HelmCreds{InsecureSkipVerify: true}, false, "", "", WithIndexCacheScopedByCreds(privateCache))
	_, err := anonClient.GetIndex(t.Context(), false, 1024*1024)
	require.NoError(t, err)

	// A different credential scope must not consume the anonymous entry and,
	// on publication, must not overwrite it either.
	authedCache := &recordingIndexCache{}
	authedClient := NewClient(server.URL, HelmCreds{Username: user, Password: pass, InsecureSkipVerify: true}, false, "", "", WithIndexCacheScopedByCreds(authedCache))
	index, err := authedClient.GetIndex(t.Context(), false, 1024*1024)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", index.Entries["fake"][0].Version)

	// The two scopes must have used different cache keys.
	anonKey, err := (&nativeHelmChart{repoURL: server.URL, scopedByCreds: true}).cacheRepoKeyScoped(t.Context(), "")
	require.NoError(t, err)
	authedKey, err := (&nativeHelmChart{repoURL: server.URL, scopedByCreds: true}).cacheRepoKeyScoped(t.Context(), credsScope(HelmCreds{Username: user, Password: pass}))
	require.NoError(t, err)
	assert.NotEqual(t, anonKey, authedKey)
	assert.NotContains(t, anonKey, pass)
	assert.NotContains(t, authedKey, pass)
}

func TestIndexBadBodyIsNotCached(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("ETag", `"bad-index"`)
		_, _ = w.Write([]byte("this is not a helm index"))
	}))
	cacheHeld := &recordingIndexCache{}
	client := NewClient(server.URL, HelmCreds{InsecureSkipVerify: true}, false, "", "", WithIndexCache(cacheHeld))

	_, err := client.GetIndex(t.Context(), false, 1024*1024)
	require.Error(t, err, "format validation must reject non-index bodies")
	assert.Equal(t, int64(0), cacheHeld.sets.Load(), "invalid index bodies must never be published")
}

func TestIndexUnusable304RetriesUnconditionally(t *testing.T) {
	requests := atomic.Int64{}
	// The origin answers the conditional request with an unusable 304 (no
	// matching ETag semantics): the client must fall back to a full GET.
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requests.Add(1)
		if n == 1 {
			w.Header().Set("ETag", `"etag"`)
			_, _ = w.Write(newIndexBody("1.0.0"))
			return
		}
		if r.Header.Get("If-None-Match") != "" {
			// Malicious/incorrect 304: its ETag does not match the cached
			// representation the validator was sent for, so the response
			// validates nothing the client asked about. A bare 304 (no ETag
			// header) would be an "omitted" validator and stays usable.
			w.Header().Set("ETag", `"mismatched-etag"`)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", fmt.Sprintf(`"etag-%d"`, n))
		_, _ = w.Write(newIndexBody("2.0.0"))
	}))
	cacheHeld := &recordIndexCache{expiration: time.Minute}
	client := NewClient(server.URL, HelmCreds{InsecureSkipVerify: true}, false, "", "", WithIndexCache(cacheHeld))

	index, err := client.GetIndex(t.Context(), false, 1024*1024)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", index.Entries["fake"][0].Version)

	cacheHeld.mu.Lock()
	stale := *cacheHeld.record
	stale.StoredAt = time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339Nano)
	cacheHeld.record = &stale
	cacheHeld.mu.Unlock()

	index2, err := client.GetIndex(t.Context(), false, 1024*1024)
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", index2.Entries["fake"][0].Version, "an unusable 304 must be retried unconditionally")
	assert.Equal(t, int64(3), requests.Load(), "expected: initial GET + unusable 304 + unconditional retry")
}

func TestIndexSplitCacheUsesBoundValidatorsOnly(t *testing.T) {
	// Validators bound in-process to one body must never 304-validate a
	// different body that reached the cache through another writer.
	etag := `"split-etag"`
	bodies := map[string][]byte{
		"1": newIndexBody("1.0.0"),
		"2": newIndexBody("2.0.0"),
	}
	var served atomic.Int64
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := served.Add(1)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		if n <= 2 {
			_, _ = w.Write(bodies["1"])
			return
		}
		_, _ = w.Write(bodies["2"])
	}))
	cacheHeld := &recordIndexCache{expiration: time.Minute}
	client := NewClient(server.URL, HelmCreds{InsecureSkipVerify: true}, false, "", "", WithIndexCache(cacheHeld))

	_, err := client.GetIndex(t.Context(), false, 1024*1024)
	require.NoError(t, err)

	// Replace the cache contents out-of-band (another replica published a
	// different body under the same key).
	cacheHeld.mu.Lock()
	replaced := *cacheHeld.record
	replaced.Body = bytes.Clone(bodies["2"])
	replaced.BodyDigest = ""
	replaced.StoredAt = time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339Nano)
	cacheHeld.record = &replaced
	cacheHeld.mu.Unlock()

	// The stale-bound validators must not be paired with the new bytes.
	_, err = client.GetIndex(t.Context(), false, 1024*1024)
	require.NoError(t, err)
	index, err := client.GetIndex(t.Context(), false, 1024*1024)
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", index.Entries["fake"][0].Version)
	assert.Greater(t, served.Load(), int64(2), "validators bound to the old body must not 304-validate the new body")
}

func TestIndexRecordRoundTripThroughClient(t *testing.T) {
	// With a full record store (atomic body+validators), the client must
	// publish one record on the 200 and zero records on the 304.
	etag := `"record-etag"`
	origin := atomic.Int64{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := origin.Add(1)
		if n > 1 && r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		_, _ = w.Write(newIndexBody("1.0.0"))
	}))
	cacheHeld := &recordingIndexCache{}
	client := NewClient(server.URL, HelmCreds{InsecureSkipVerify: true}, false, "", "", WithIndexCache(cacheHeld))

	_, err := client.GetIndex(t.Context(), false, 1024*1024)
	require.NoError(t, err)
	require.Equal(t, int64(1), cacheHeld.sets.Load())
	_, err = client.GetIndex(t.Context(), false, 1024*1024)
	require.NoError(t, err)
	assert.Equal(t, int64(1), cacheHeld.sets.Load(), "304 must not republish")
	assert.Less(t, origin.Load(), int64(3), "second fetch should be a single conditional request")
	_ = time.Now // silence unused import if helpers change
}

func TestNewTLSConfigRejectsEmptyCAFile(t *testing.T) {
	emptyCA := t.TempDir() + "/empty-ca.pem"
	require.NoError(t, os.WriteFile(emptyCA, nil, 0o600))

	_, err := newTLSConfig(HelmCreds{CAPath: emptyCA})
	require.Error(t, err, "an empty configured CA bundle must fail closed instead of falling back to system trust")

	_, err = newTLSConfig(HelmCreds{CAPath: t.TempDir() + "/missing-ca.pem"})
	require.Error(t, err)

	// No custom CA configured: system trust (nil RootCAs) is the default.
	tlsConfig, err := newTLSConfig(HelmCreds{})
	require.NoError(t, err)
	require.Nil(t, tlsConfig.RootCAs)
}
