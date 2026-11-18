package helm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	keysync "github.com/argoproj/pkg/v2/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	utilio "github.com/argoproj/argo-cd/v3/util/io"
	utiltls "github.com/argoproj/argo-cd/v3/util/tls"
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

		client := NewClient("https://argoproj.github.io/argo-helm", HelmCreds{}, false, "", "", WithIndexCache(&fakeIndexCache{data: data.Bytes()}))
		index, err := client.GetIndex(t.Context(), false, 10000)

		require.NoError(t, err)
		assert.Equal(t, fakeIndex, *index)
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

// fakeValidatorsCache is an indexCache that also implements
// indexCacheWithValidators, mirroring reposerver/cache.Cache.
type fakeValidatorsCache struct {
	fakeIndexCache
	etag         string
	lastModified string
}

func (f *fakeValidatorsCache) SetHelmIndexValidators(_ string, etag string, lastModified string) error {
	f.etag = etag
	f.lastModified = lastModified
	return nil
}

func (f *fakeValidatorsCache) GetHelmIndexValidators(_ string) (string, string, error) {
	if f.etag == "" && f.lastModified == "" {
		return "", "", errors.New("validators not found")
	}
	return f.etag, f.lastModified, nil
}

type conditionalGetServer struct {
	etag         string
	lastModified string
	body         []byte
	requests     atomic.Int32
	// when true, a matching If-None-Match returns 304; otherwise 200 always.
	supports304 bool
}

func (s *conditionalGetServer) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.requests.Add(1)
		inm := r.Header.Get("If-None-Match")
		if s.supports304 && inm != "" && inm == s.etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		if s.lastModified != "" {
			w.Header().Set("Last-Modified", s.lastModified)
		}
		if s.etag != "" {
			w.Header().Set("ETag", s.etag)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(s.body)
	}
}

func newIndex(t *testing.T) []byte {
	t.Helper()
	fakeIndex := Index{Entries: map[string]Entries{"fake": {}}}
	data := bytes.Buffer{}
	require.NoError(t, yaml.NewEncoder(&data).Encode(fakeIndex))
	return data.Bytes()
}

func TestConditionalGET_Revalidates(t *testing.T) {
	body := newIndex(t)
	srv := &conditionalGetServer{etag: `"v1"`, body: body, supports304: true}
	httpSrv := httptest.NewServer(srv.handler())
	defer httpSrv.Close()

	cache := &fakeIndexCache{}
	client := NewClient(httpSrv.URL, HelmCreds{}, false, "", "", WithIndexCache(cache))

	// First fetch: 200, validators stored.
	_, err := client.GetIndex(t.Context(), true, 10000)
	require.NoError(t, err)
	assert.Equal(t, int32(1), srv.requests.Load())
	etag, _ := sharedInMemoryValidators.getForBody(httpSrv.URL, cache.data)
	assert.Equal(t, `"v1"`, etag)

	// Second fetch: If-None-Match sent, 304 received, cached body returned —
	// still only 2 requests total, no re-download.
	idx, err := client.GetIndex(t.Context(), true, 10000)
	require.NoError(t, err)
	assert.Equal(t, int32(2), srv.requests.Load())
	assert.Equal(t, body, cache.data)
	assert.NotNil(t, idx)
}

func TestConditionalGET_LastModifiedOnly(t *testing.T) {
	body := newIndex(t)
	srv := &conditionalGetServer{lastModified: "Wed, 21 Oct 2026 07:28:00 GMT", body: body, supports304: true}
	httpSrv := httptest.NewServer(srv.handler())
	defer httpSrv.Close()

	cache := &fakeIndexCache{}
	client := NewClient(httpSrv.URL, HelmCreds{}, false, "", "", WithIndexCache(cache))

	_, err := client.GetIndex(t.Context(), true, 10000)
	require.NoError(t, err)
	_, err = client.GetIndex(t.Context(), true, 10000)
	require.NoError(t, err)
	assert.Equal(t, int32(2), srv.requests.Load())
}

func TestConditionalGET_NoValidatorSupport(t *testing.T) {
	// Server sends no ETag/Last-Modified: every fetch is a plain 200 GET.
	body := newIndex(t)
	srv := &conditionalGetServer{body: body}
	httpSrv := httptest.NewServer(srv.handler())
	defer httpSrv.Close()

	cache := &fakeIndexCache{}
	client := NewClient(httpSrv.URL, HelmCreds{}, false, "", "", WithIndexCache(cache))

	for range 3 {
		_, err := client.GetIndex(t.Context(), true, 10000)
		require.NoError(t, err)
	}
	assert.Equal(t, int32(3), srv.requests.Load())
}

func TestConditionalGET_IndexChanged(t *testing.T) {
	// 200 with a new body and new ETag replaces both body and validators.
	srv := &conditionalGetServer{etag: `"v1"`, body: newIndex(t), supports304: true}
	var handler atomic.Pointer[http.HandlerFunc]
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h := handler.Load(); h != nil {
			(*h)(w, r)
			return
		}
		srv.handler()(w, r)
	})
	httpSrv := httptest.NewServer(wrapped)
	defer httpSrv.Close()

	cache := &fakeIndexCache{}
	client := NewClient(httpSrv.URL, HelmCreds{}, false, "", "", WithIndexCache(cache))

	_, err := client.GetIndex(t.Context(), true, 10000)
	require.NoError(t, err)

	// Server now has a new index with a new ETag; 304 support for the old
	// ETag is gone, so a conditional request gets a fresh 200.
	srv.etag = `"v2"`
	srv.body = newIndex(t)
	newBody := append([]byte(nil), srv.body...)
	srv.body = newBody
	srv.supports304 = false

	idx, err := client.GetIndex(t.Context(), true, 10000)
	require.NoError(t, err)
	assert.Equal(t, int32(2), srv.requests.Load())
	etag, _ := sharedInMemoryValidators.getForBody(httpSrv.URL, cache.data)
	assert.Equal(t, `"v2"`, etag)
	assert.Equal(t, newBody, cache.data)
	require.NotNil(t, idx)
}

func TestApplyConditionalHeaders(t *testing.T) {
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.com/index.yaml", http.NoBody)
	applyConditionalHeaders(req, `"abc"`, "Wed, 21 Oct 2026 07:28:00 GMT")
	// ETag present: only If-None-Match is sent (RFC 9110 §13.1.2).
	assert.Equal(t, `"abc"`, req.Header.Get("If-None-Match"))
	assert.Empty(t, req.Header.Get("If-Modified-Since"))

	req2, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.com/index.yaml", http.NoBody)
	applyConditionalHeaders(req2, "", "Wed, 21 Oct 2026 07:28:00 GMT")
	assert.Empty(t, req2.Header.Get("If-None-Match"))
	assert.Equal(t, "Wed, 21 Oct 2026 07:28:00 GMT", req2.Header.Get("If-Modified-Since"))
}

func TestInMemoryIndexValidators(t *testing.T) {
	v := newInMemoryIndexValidators()
	_, _, err := v.GetHelmIndexValidators("repo")
	require.Error(t, err)
	require.NoError(t, v.SetHelmIndexValidators("repo", `"e"`, "lm"))
	etag, lm, err := v.GetHelmIndexValidators("repo")
	require.NoError(t, err)
	assert.Equal(t, `"e"`, etag)
	assert.Equal(t, "lm", lm)
}

func TestValidatorsFor_InMemoryFallback(t *testing.T) {
	// A cache implementing only the base interface gets the in-process
	// fallback (never nil).
	v := validatorsFor(&fakeIndexCache{})
	require.NotNil(t, v)
	// A cache implementing the extension is used directly.
	ext := validatorsFor(&fakeValidatorsCache{})
	assert.Same(t, sharedInMemoryValidators, validatorsFor(&fakeIndexCache{}))
	_ = ext
}

func TestConditionalGET_304WithoutCachedBody(t *testing.T) {
	// Validators survive but the cached body was evicted: 304 → one retry
	// without validators → 200 body.
	body := newIndex(t)
	srv := &conditionalGetServer{etag: `"v1"`, body: body, supports304: true}
	httpSrv := httptest.NewServer(srv.handler())
	defer httpSrv.Close()

	cache := &fakeIndexCache{}
	client := NewClient(httpSrv.URL, HelmCreds{}, false, "", "", WithIndexCache(cache))

	// Prime validators.
	_, err := client.GetIndex(t.Context(), true, 10000)
	require.NoError(t, err)

	// Evict the body but keep validators. Conditional headers are only sent
	// when a usable cached body exists, so this degrades to a plain 200 GET
	// (loadRepoIndex keeps a defensive one-shot unconditional retry for an
	// orphan 304 — unreachable through this path, but protects against a
	// body-evicting cache change).
	cache.data = nil
	idx, err := client.GetIndex(t.Context(), true, 10000)
	require.NoError(t, err)
	assert.Equal(t, int32(2), srv.requests.Load())
	assert.Equal(t, body, cache.data)
	require.NotNil(t, idx)
}

func upstreamIndex(version int32) []byte {
	return []byte(fmt.Sprintf("apiVersion: v1\nentries:\n  child:\n    - version: %d.0.0\n      urls: [child-%d.0.0.tgz]\n", version, version))
}

func TestIndexUpstreamValidatorChanges(t *testing.T) {
	for _, mode := range []string{"etag", "weak etag", "last modified", "both", "none"} {
		t.Run(mode, func(t *testing.T) {
			var version atomic.Int32
			version.Store(1)
			type request struct {
				etag, modified string
				status         int
			}
			requests := make(chan request, 8)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				generation := version.Load()
				etag, modified := "", ""
				if mode == "etag" || mode == "both" || mode == "weak etag" {
					etag = fmt.Sprintf(`"v%d"`, generation)
					if mode == "weak etag" {
						etag = "W/" + etag
					}
					w.Header().Set("ETag", etag)
				}
				if mode == "last modified" || mode == "both" {
					modified = time.Date(2026, 1, 1, int(generation), 0, 0, 0, time.UTC).Format(http.TimeFormat)
					w.Header().Set("Last-Modified", modified)
				}
				status := http.StatusOK
				if etag != "" && r.Header.Get("If-None-Match") == etag ||
					etag == "" && modified != "" && r.Header.Get("If-Modified-Since") == modified {
					status = http.StatusNotModified
				}
				requests <- request{r.Header.Get("If-None-Match"), r.Header.Get("If-Modified-Since"), status}
				w.WriteHeader(status)
				if status == http.StatusOK {
					_, _ = w.Write(upstreamIndex(generation))
				}
			}))
			t.Cleanup(server.Close)
			store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
			client := regressionClient(server.URL)
			fetch := func(want int32) {
				t.Helper()
				body, err := store.Revalidate(t.Context(), "upstream", client)
				require.NoError(t, err)
				require.Equal(t, upstreamIndex(want), body)
			}
			observe := func() request {
				t.Helper()
				select {
				case got := <-requests:
					return got
				default:
					t.Fatal("expected one completed origin request")
					return request{}
				}
			}
			fetch(1)
			require.Equal(t, http.StatusOK, observe().status)
			fetch(1)
			require.Empty(t, requests, "a warm read must not check upstream")
			regressionExpire(t, store, "upstream")
			fetch(1)
			unchanged := observe()
			wantStatus := http.StatusNotModified
			if mode == "none" {
				wantStatus = http.StatusOK
			}
			require.Equal(t, wantStatus, unchanged.status)
			if mode == "last modified" {
				require.NotEmpty(t, unchanged.modified)
				require.Empty(t, unchanged.etag)
			}
			if mode == "both" {
				require.NotEmpty(t, unchanged.etag)
				require.Empty(t, unchanged.modified, "ETag takes precedence")
			}
			version.Store(2)
			regressionExpire(t, store, "upstream")
			fetch(2)
			changed := observe()
			require.Equal(t, http.StatusOK, changed.status)
			require.Equal(t, unchanged.etag, changed.etag)
			require.Equal(t, unchanged.modified, changed.modified)
			regressionExpire(t, store, "upstream")
			fetch(2)
			revalidated := observe()
			require.Equal(t, wantStatus, revalidated.status)
			if mode != "none" {
				require.NotEqual(t, unchanged, revalidated, "the new response's validators must replace the old generation")
			}
		})
	}
}

func TestIndexUpstreamOrphan304RetryIsBounded(t *testing.T) {
	for _, repeat304 := range []bool{false, true} {
		t.Run(fmt.Sprintf("repeat304=%t", repeat304), func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if requests.Add(1) == 1 || repeat304 {
					w.WriteHeader(http.StatusNotModified)
					return
				}
				_, _ = w.Write(upstreamIndex(2))
			}))
			t.Cleanup(server.Close)
			body, err := regressionClient(server.URL).loadRepoIndex(t.Context(), 1<<20)
			if repeat304 {
				require.ErrorContains(t, err, "304")
				require.Empty(t, body)
			} else {
				require.NoError(t, err)
				require.Equal(t, upstreamIndex(2), body)
			}
			require.EqualValues(t, 2, requests.Load(), "an unusable 304 must retry exactly once")
		})
	}
}

func TestIndexUpstreamRedirectRetainsConditionalRequest(t *testing.T) {
	requests := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/index.yaml" {
			http.Redirect(w, r, "/actual/index.yaml", http.StatusTemporaryRedirect)
			return
		}
		requests <- r.Header.Get("If-None-Match")
		w.Header().Set("ETag", `"redirected"`)
		if r.Header.Get("If-None-Match") == `"redirected"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = w.Write(upstreamIndex(1))
	}))
	t.Cleanup(server.Close)
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	for range 2 {
		body, err := store.Revalidate(t.Context(), "redirect", regressionClient(server.URL))
		require.NoError(t, err)
		require.Equal(t, upstreamIndex(1), body)
		regressionExpire(t, store, "redirect")
	}
	require.Empty(t, <-requests)
	require.Equal(t, `"redirected"`, <-requests)
}

func TestIndexUpstreamValidationThroughProxy(t *testing.T) {
	for _, policy := range []string{"expired", "always fresh", "forced"} {
		t.Run(policy, func(t *testing.T) {
			var originRequests atomic.Int32
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				originRequests.Add(1)
				w.Header().Set("ETag", `"new"`)
				_, _ = w.Write(upstreamIndex(2))
			}))
			t.Cleanup(origin.Close)
			gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// A proxy may satisfy an unconditional request using its own
				// still-fresh response unless origin validation is requested.
				if !strings.Contains(r.Header.Get("Cache-Control"), "no-cache") {
					w.Header().Set("ETag", `"old"`)
					_, _ = w.Write(upstreamIndex(1))
					return
				}
				req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, origin.URL+"/index.yaml", http.NoBody)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadGateway)
					return
				}
				req.Header = r.Header.Clone()
				response, err := origin.Client().Do(req)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadGateway)
					return
				}
				defer func() { _ = response.Body.Close() }()
				w.Header().Set("ETag", response.Header.Get("ETag"))
				w.WriteHeader(response.StatusCode)
				_, _ = io.Copy(w, response.Body)
			}))
			t.Cleanup(gateway.Close)
			store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
			require.NoError(t, store.SetWithValidators("proxy", upstreamIndex(1), `"old"`, ""))
			client := NewClient("http://charts.example.invalid", HelmCreds{}, false, gateway.URL, "").(*nativeHelmChart)
			var body []byte
			var err error
			switch policy {
			case "expired":
				regressionExpire(t, store, "proxy")
				body, err = store.Revalidate(t.Context(), "proxy", client)
			case "always fresh":
				client.indexCache = retainedIndexView{&HelmIndexRecord{Body: upstreamIndex(1), ETag: `"old"`}}
				body, err = client.loadRepoIndex(t.Context(), 1<<20)
			case "forced":
				body, err = store.ForceRefresh(t.Context(), "proxy", client)
			}
			require.NoError(t, err)
			require.Equal(t, upstreamIndex(2), body, "freshness checks must reach the origin through a caching proxy")
			require.EqualValues(t, 1, originRequests.Load())
		})
	}
}

func TestIndexOCIChangedTagsAndAuthentication(t *testing.T) {
	var version, tagRequests, indexRequests atomic.Int32
	var authorized atomic.Bool
	version.Store(1)
	authorized.Store(true)
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "index.yaml") {
			indexRequests.Add(1)
		}
		username, password, ok := r.BasicAuth()
		if !authorized.Load() || !ok || username != "reader" || password != "secret" {
			w.Header().Set("WWW-Authenticate", `Basic realm="registry"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/v2/child/tags/list" {
			tagRequests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"name": "child", "tags": []string{fmt.Sprintf("%d.0.0", version.Load())}})
			return
		}
		if r.URL.Path != "/v2/" {
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(registry.Close)
	cache := &fakeIndexCache{}
	client := NewClient(strings.TrimPrefix(registry.URL, "http://"), HelmCreds{Username: "reader", Password: "secret"}, true, "", "",
		WithPlainHTTP(), WithIndexCacheScopedByCreds(cache))
	tags, err := client.GetTags(t.Context(), "child", false)
	require.NoError(t, err)
	require.Equal(t, []string{"1.0.0"}, tags)
	version.Store(2)
	tags, err = client.GetTags(t.Context(), "child", false)
	require.NoError(t, err)
	require.Equal(t, []string{"1.0.0"}, tags, "ordinary calls retain the existing tag-cache policy")
	require.EqualValues(t, 1, tagRequests.Load())
	tags, err = client.GetTags(t.Context(), "child", true)
	require.NoError(t, err)
	require.Equal(t, []string{"2.0.0"}, tags, "forced tag resolution must observe newly published versions")
	require.EqualValues(t, 2, tagRequests.Load())
	version.Store(3)
	cache.data = nil // Simulate tag-cache eviction/expiry.
	tags, err = client.GetTags(t.Context(), "child", false)
	require.NoError(t, err)
	require.Equal(t, []string{"3.0.0"}, tags)
	require.EqualValues(t, 3, tagRequests.Load())
	authorized.Store(false)
	_, err = client.GetTags(t.Context(), "child", true)
	require.ErrorContains(t, err, "401", "a warmed tag cache must not mask forced authentication failures")
	require.Zero(t, indexRequests.Load(), "OCI tag discovery must never request index.yaml")
}

type credentialIndexCache struct{ bodies map[string][]byte }

func (c *credentialIndexCache) GetHelmIndex(key string, body *[]byte) error {
	data, ok := c.bodies[key]
	if !ok {
		return errors.New("cache miss")
	}
	*body = append([]byte(nil), data...)
	return nil
}

func (c *credentialIndexCache) SetHelmIndex(key string, body []byte) error {
	c.bodies[key] = append([]byte(nil), body...)
	return nil
}

func TestConditionalGET_BaseCacheRetainsValidators(t *testing.T) {
	var conditional atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"unchanged"` {
			conditional.Add(1)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"unchanged"`)
		_, _ = w.Write([]byte(regressionIndexBody))
	}))
	t.Cleanup(server.Close)
	client := NewClient(server.URL, HelmCreds{}, false, "", "", WithIndexCache(&fakeIndexCache{}))
	_, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	_, err = client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	require.EqualValues(t, 1, conditional.Load(), "base-interface cache must publish to the in-memory validator fallback")
}

func TestConditionalGET_BaseCacheRejectsMismatchedGeneration(t *testing.T) {
	var headers []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = append(headers, r.Header.Get("If-None-Match"))
		w.Header().Set("ETag", `"origin"`)
		_, _ = w.Write([]byte(regressionIndexBody))
	}))
	t.Cleanup(server.Close)
	cache := &fakeIndexCache{}
	client := NewClient(server.URL, HelmCreds{}, false, "", "", WithIndexCache(cache))
	_, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	// Another writer changes the body without publishing a paired validator.
	cache.data = []byte(strings.ReplaceAll(regressionIndexBody, "0.1.0", "0.2.0") + "\n")
	_, err = client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	require.Equal(t, []string{"", ""}, headers, "validators must match the exact cached bytes")
}

func TestGetTagsDockerFallbackCredentialIsolation(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		username, password, ok := r.BasicAuth()
		if !ok || username != "reader" || password != "test-secret" {
			w.Header().Set("WWW-Authenticate", `Basic realm="registry"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"child","tags":["private-version"]}`))
	}))
	t.Cleanup(server.Close)
	host := strings.TrimPrefix(server.URL, "http://")
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)
	config, _ := json.Marshal(map[string]any{"auths": map[string]any{host: map[string]string{"auth": base64.StdEncoding.EncodeToString([]byte("reader:test-secret"))}}})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), config, 0o600))
	cache := &credentialIndexCache{bodies: map[string][]byte{}}
	first := NewClient(host, HelmCreds{}, true, "", "", WithPlainHTTP(), WithIndexCacheScopedByCreds(cache))
	tags, err := first.GetTags(t.Context(), "child", false)
	require.NoError(t, err)
	require.Equal(t, []string{"private-version"}, tags)
	before := requests.Load()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"auths":{}}`), 0o600))
	second := NewClient(host, HelmCreds{}, true, "", "", WithPlainHTTP(), WithIndexCacheScopedByCreds(cache))
	_, err = second.GetTags(t.Context(), "child", false)
	require.Error(t, err, "anonymous operation must not read Docker-authenticated private tag metadata")
	assert.Greater(t, requests.Load(), before, "the second authentication context must reach the origin")
}

func TestGetTagsDockerHubAliases(t *testing.T) {
	t.Setenv("DOCKER_CONFIG", t.TempDir())
	config := `{"auths":{"https://index.docker.io/v1/":{"auth":"cmVhZGVyOnRlc3Qtc2VjcmV0"}}}`
	require.NoError(t, os.WriteFile(filepath.Join(os.Getenv("DOCKER_CONFIG"), "config.json"), []byte(config), 0o600))
	var authenticated atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Host != "registry-1.docker.io" {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		user, password, _ := r.BasicAuth()
		if user != "reader" || password != "test-secret" {
			w.Header().Set("WWW-Authenticate", `Basic realm="registry"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		authenticated.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"name":"private/child","tags":["1.0.0"]}`)
	}))
	t.Cleanup(server.Close)
	for _, host := range []string{"docker.io", "registry-1.docker.io"} {
		t.Run(host, func(t *testing.T) {
			client := NewClient(host, HelmCreds{}, true, server.URL, "", WithPlainHTTP())
			tags, err := client.GetTags(t.Context(), "private/child", true)
			require.NoError(t, err)
			require.Equal(t, []string{"1.0.0"}, tags)
		})
	}
	require.EqualValues(t, 2, authenticated.Load())
}

func TestGetTagsDockerHelperFailure(t *testing.T) {
	t.Setenv("DOCKER_CONFIG", t.TempDir())
	require.NoError(t, os.WriteFile(filepath.Join(os.Getenv("DOCKER_CONFIG"), "config.json"), []byte(`{"credsStore":"argocd-definitely-missing-test-helper"}`), 0o600))
	for _, challenge := range []bool{false, true} {
		t.Run(strconv.FormatBool(challenge), func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				if challenge {
					w.Header().Set("WWW-Authenticate", `Basic realm="registry"`)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"name":"child","tags":["public-version"]}`)
			}))
			t.Cleanup(server.Close)
			cache := &credentialIndexCache{bodies: map[string][]byte{}}
			client := NewClient(strings.TrimPrefix(server.URL, "http://"), HelmCreds{}, true, "", "", WithPlainHTTP(), WithIndexCacheScopedByCreds(cache))
			tags, err := client.GetTags(t.Context(), "child", false)
			if challenge {
				require.ErrorContains(t, err, "argocd-definitely-missing-test-helper")
			} else {
				require.NoError(t, err)
				require.Equal(t, []string{"public-version"}, tags)
			}
			require.Positive(t, requests.Load(), "unknown credentials must reach the origin without consulting shared metadata")
			require.Empty(t, cache.bodies, "unknown credentials must never populate an anonymous shared-cache identity")
		})
	}
}

func TestGetTagsDockerCredentialsPreserveExplicitScope(t *testing.T) {
	t.Setenv("DOCKER_CONFIG", t.TempDir())
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, _ := r.BasicAuth()
		if username != "reader" || password != "test-secret" {
			w.Header().Set("WWW-Authenticate", `Basic realm="registry"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"name":"child","tags":["1.0.0"]}`)
	}))
	t.Cleanup(server.Close)
	host := strings.TrimPrefix(server.URL, "http://")
	config, err := json.Marshal(map[string]any{"auths": map[string]any{host: map[string]string{"auth": "cmVhZGVyOnRlc3Qtc2VjcmV0"}}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(os.Getenv("DOCKER_CONFIG"), "config.json"), config, 0o600))
	cache := &credentialIndexCache{bodies: map[string][]byte{}}
	for _, scope := range []string{"first", "second", "first"} {
		client := NewClient(host, HelmCreds{}, true, "", "", WithPlainHTTP(), WithIndexCache(cache), WithIndexCacheScope(scope))
		_, err := client.GetTags(t.Context(), "child", false)
		require.NoError(t, err)
	}
	require.EqualValues(t, 2, requests.Load(), "each explicit partition must fetch once")
	require.Len(t, cache.bodies, 2)
}

func TestConditionalGET_Unsolicited304WithCachedBody(t *testing.T) {
	for _, retryOK := range []bool{false, true} {
		t.Run(strconv.FormatBool(retryOK), func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if requests.Add(1) == 2 && retryOK {
					_, _ = w.Write(upstreamIndex(2))
					return
				}
				w.WriteHeader(http.StatusNotModified)
			}))
			t.Cleanup(server.Close)
			cache := &fakeIndexCache{data: []byte(regressionIndexBody)}
			client := NewClient(server.URL, HelmCreds{}, false, "", "", WithIndexCache(cache))
			index, err := client.GetIndex(t.Context(), true, 1<<20)
			if retryOK {
				require.NoError(t, err)
				require.Equal(t, "2.0.0", index.Entries["child"][0].Version, "the retry must return the new response body")
			} else {
				require.Error(t, err)
			}
			require.EqualValues(t, 2, requests.Load(), "a body without a sent validator cannot authorize a 304")
		})
	}
}

func TestHelmCAFileRotationCannotUsePreviousTrust(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, regressionIndexBody) }))
	t.Cleanup(server.Close)
	ca := filepath.Join(t.TempDir(), "ca.pem")
	oldPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	require.NoError(t, os.WriteFile(ca, oldPEM, 0o600))
	creds := HelmCreds{CAPath: ca}
	oldScope := credsScope(creds)
	frozen, err := snapshotCreds(creds)
	require.NoError(t, err)
	cache := &credentialIndexCache{bodies: map[string][]byte{}}
	client := NewClient(server.URL, creds, false, "", "", WithIndexCacheScopedByCreds(cache))
	_, err = client.GetIndex(t.Context(), false, 1<<20)
	require.NoError(t, err)
	different, err := utiltls.GenerateHealthCheckClientCert()
	require.NoError(t, err)
	newPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: different.Certificate[0]})
	require.NoError(t, os.WriteFile(ca, newPEM, 0o600))
	assert.NotEqual(t, oldScope, credsScope(creds), "Helm HTTP cache identity must include CA contents, as native OCI does")
	config, err := newTLSConfig(frozen)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM(oldPEM))
	require.True(t, config.RootCAs.Equal(pool), "the HTTP transport must keep its snapshotted CA")
	again, err := snapshotCreds(frozen)
	require.NoError(t, err)
	require.Equal(t, oldScope, credsScope(again), "resnapshotting must not reread rotated CA material")
	cmd := newTestCmd(t)
	entries, err := buildRepoEntries(&helm{cmd: *cmd}, HelmRepository{Name: "charts", Repo: server.URL, Creds: frozen})
	require.NoError(t, err)
	copied, err := os.ReadFile(entries[0].CAFile)
	require.NoError(t, err)
	require.Equal(t, oldPEM, copied, "the Helm subprocess must use the same frozen CA as the index request")
	_, err = client.GetIndex(t.Context(), false, 1<<20)
	require.Error(t, err, "a warm index cannot bypass the changed TLS trust context")
	require.NoError(t, os.WriteFile(ca, nil, 0o600))
	emptyCA, err := snapshotCreds(creds)
	require.NoError(t, err, "credential snapshots preserve existing non-prewarm behavior")
	_, err = buildRepoEntries(&helm{cmd: *cmd}, HelmRepository{Name: "charts", Repo: server.URL, Creds: emptyCA})
	require.ErrorContains(t, err, "empty", "prewarming cannot let Helm replace empty custom trust with system roots")
}

type nonAtomicRecordCache struct {
	mu     sync.Mutex
	record *HelmIndexRecord
	writes int
}

func (c *nonAtomicRecordCache) GetHelmIndex(_ string, b *[]byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	*b = bytes.Clone(c.record.Body)
	return nil
}

func (c *nonAtomicRecordCache) SetHelmIndex(_ string, b []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writes++
	c.record = &HelmIndexRecord{Body: bytes.Clone(b)}
	return nil
}

func (c *nonAtomicRecordCache) GetHelmIndexRetainedRecord(string) (*HelmIndexRecord, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	r := *c.record
	r.Body = bytes.Clone(r.Body)
	return &r, nil
}

func (c *nonAtomicRecordCache) SetHelmIndexRecord(_ string, b []byte, e, m string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writes++
	c.record = &HelmIndexRecord{Body: bytes.Clone(b), ETag: e, LastModified: m}
	return nil
}

type nonAtomicValidatorCache struct {
	indexCache
	records *nonAtomicRecordCache
}

func (c *nonAtomicValidatorCache) GetHelmIndexValidators(string) (string, string, error) {
	c.records.mu.Lock()
	defer c.records.mu.Unlock()
	return c.records.record.ETag, c.records.record.LastModified, nil
}

func (c *nonAtomicValidatorCache) SetHelmIndexValidators(_ string, etag, modified string) error {
	c.records.mu.Lock()
	defer c.records.mu.Unlock()
	c.records.writes++
	c.records.record.ETag, c.records.record.LastModified = etag, modified
	return nil
}

func TestMetadataCacheWithoutComparisonCannotPublish(t *testing.T) {
	for _, kind := range []string{"records", "validators only"} {
		t.Run(kind, func(t *testing.T) {
			cache := &nonAtomicRecordCache{record: &HelmIndexRecord{Body: upstreamIndex(0)}}
			var adapter indexCache = cache
			if kind == "validators only" {
				adapter = &nonAtomicValidatorCache{indexCache: cache, records: cache}
			}
			entered, release := make(chan struct{}), make(chan struct{})
			resume := sync.OnceFunc(func() { close(release) })
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				close(entered)
				select {
				case <-release:
				case <-r.Context().Done():
					return
				}
				_, _ = w.Write(upstreamIndex(1))
			}))
			t.Cleanup(server.Close)
			t.Cleanup(resume)
			client := NewClient(server.URL, HelmCreds{}, false, "", "", WithIndexCache(adapter))
			done := make(chan error, 1)
			go func() { _, err := client.GetIndex(t.Context(), true, 1<<20); done <- err }()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("fetch never started")
			}
			cache.mu.Lock()
			cache.record = &HelmIndexRecord{Body: upstreamIndex(2)}
			cache.mu.Unlock()
			resume()
			select {
			case err := <-done:
				require.NoError(t, err)
			case <-time.After(5 * time.Second):
				t.Fatal("fetch never finished")
			}
			cache.mu.Lock()
			defer cache.mu.Unlock()
			require.Zero(t, cache.writes, "a versioned cache must not fall back to unguarded publication")
			require.Equal(t, upstreamIndex(2), cache.record.Body)
		})
	}
}

func TestConditionalGET304CannotChangeBodyIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, cached, received string
		retry                  bool
	}{
		{"last modified gains strong tag", "", `"new"`, true},
		{"different strong tag", `"old"`, `"new"`, true},
		{"weak cannot authorize strong", `W/"same"`, `"same"`, true},
		{"different weak tag", `W/"old"`, `W/"new"`, true},
		{"matching strong", `"same"`, `"same"`, false},
		{"matching weak", `W/"same"`, `W/"same"`, false},
		{"strong becomes weak", `"same"`, `W/"same"`, false},
		{"omitted tag", `"same"`, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if requests.Add(1) == 1 {
					if tc.cached != "" {
						assert.Equal(t, tc.cached, r.Header.Get("If-None-Match"))
					} else {
						assert.NotEmpty(t, r.Header.Get("If-Modified-Since"))
					}
					if tc.received != "" {
						w.Header().Set("ETag", tc.received)
					}
					w.WriteHeader(http.StatusNotModified)
					return
				}
				assert.Empty(t, r.Header.Get("If-None-Match"))
				assert.Empty(t, r.Header.Get("If-Modified-Since"))
				w.Header().Set("ETag", `"new"`)
				_, _ = w.Write(upstreamIndex(2))
			}))
			t.Cleanup(server.Close)
			record := &HelmIndexRecord{Body: upstreamIndex(1), ETag: tc.cached, LastModified: "Wed, 21 Oct 2026 07:28:00 GMT", URLIdentity: IndexURLIdentity(server.URL + "/index.yaml")}
			client := NewClient(server.URL, HelmCreds{}, false, "", "", WithIndexCache(retainedIndexView{record})).(*nativeHelmChart)
			body, err := client.loadRepoIndex(t.Context(), 1<<20)
			require.NoError(t, err)
			if tc.retry {
				require.Equal(t, upstreamIndex(2), body)
				require.EqualValues(t, 2, requests.Load())
			} else {
				require.Equal(t, record.Body, body)
				require.EqualValues(t, 1, requests.Load())
			}
		})
	}
}

func TestExtractChartCredentialFailureRemovesTemporaryHome(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TMPDIR", root)
	t.Setenv("TMP", root)
	t.Setenv("TEMP", root)
	client := NewClient("registry.example.com/charts", HelmCreds{CAPath: filepath.Join(root, "missing-ca.pem")}, true, "", "")
	_, closer, err := client.ExtractChart(t.Context(), "child", "1.0.0", false, 1<<20, false)
	require.ErrorContains(t, err, "CA file")
	require.Nil(t, closer)
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	require.Empty(t, entries, "failed extraction must leave no directory owned by a missing closer")
}
