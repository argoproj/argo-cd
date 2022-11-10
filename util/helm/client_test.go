package helm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"net/http"
	"net/http/httptest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/argoproj/argo-cd/v2/util/io"
)

type fakeIndexCache struct {
	data []byte
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
		client := NewClient("", Creds{}, false, "")
		_, err := client.GetIndex(false)
		assert.Error(t, err)
	})
	t.Run("Stable", func(t *testing.T) {
		client := NewClient("https://argoproj.github.io/argo-helm", Creds{}, false, "")
		index, err := client.GetIndex(false)
		assert.NoError(t, err)
		assert.NotNil(t, index)
	})
	t.Run("BasicAuth", func(t *testing.T) {
		client := NewClient("https://argoproj.github.io/argo-helm", Creds{
			Username: "my-password",
			Password: "my-username",
		}, false, "")
		index, err := client.GetIndex(false)
		assert.NoError(t, err)
		assert.NotNil(t, index)
	})

	t.Run("Cached", func(t *testing.T) {
		fakeIndex := Index{Entries: map[string]Entries{"fake": {}}}
		data := bytes.Buffer{}
		err := yaml.NewEncoder(&data).Encode(fakeIndex)
		require.NoError(t, err)

		client := NewClient("https://argoproj.github.io/argo-helm", Creds{}, false, "", WithIndexCache(&fakeIndexCache{data: data.Bytes()}))
		index, err := client.GetIndex(false)

		assert.NoError(t, err)
		assert.Equal(t, fakeIndex, *index)
	})

}

func Test_nativeHelmChart_ExtractChart(t *testing.T) {
	client := NewClient("https://argoproj.github.io/argo-helm", Creds{}, false, "")
	path, closer, err := client.ExtractChart("argo-cd", "0.7.1", false)
	assert.NoError(t, err)
	defer io.Close(closer)
	info, err := os.Stat(path)
	assert.NoError(t, err)
	assert.True(t, info.IsDir())
}

func Test_nativeHelmChart_ExtractChart_insecure(t *testing.T) {
	client := NewClient("https://argoproj.github.io/argo-helm", Creds{InsecureSkipVerify: true}, false, "")
	path, closer, err := client.ExtractChart("argo-cd", "0.7.1", false)
	assert.NoError(t, err)
	defer io.Close(closer)
	info, err := os.Stat(path)
	assert.NoError(t, err)
	assert.True(t, info.IsDir())
}

func Test_normalizeChartName(t *testing.T) {
	t.Run("Test non-slashed name", func(t *testing.T) {
		n := normalizeChartName("mychart")
		assert.Equal(t, n, "mychart")
	})
	t.Run("Test single-slashed name", func(t *testing.T) {
		n := normalizeChartName("myorg/mychart")
		assert.Equal(t, n, "mychart")
	})
	t.Run("Test chart name with suborg", func(t *testing.T) {
		n := normalizeChartName("myorg/mysuborg/mychart")
		assert.Equal(t, n, "mychart")
	})
	t.Run("Test double-slashed name", func(t *testing.T) {
		n := normalizeChartName("myorg//mychart")
		assert.Equal(t, n, "mychart")
	})
	t.Run("Test invalid chart name - ends with slash", func(t *testing.T) {
		n := normalizeChartName("myorg/")
		assert.Equal(t, n, "myorg/")
	})
	t.Run("Test invalid chart name - is dot", func(t *testing.T) {
		n := normalizeChartName("myorg/.")
		assert.Equal(t, n, "myorg/.")
	})
	t.Run("Test invalid chart name - is two dots", func(t *testing.T) {
		n := normalizeChartName("myorg/..")
		assert.Equal(t, n, "myorg/..")
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
		assert.NoError(t, err)
	})
	t.Run("URL with escaped characters", func(t *testing.T) {
		rawURL := fmt.Sprintf(urlTemplate, "mygroup%2Fmyproject")
		want := rawURL + "/index.yaml"
		got, err := getIndexURL(rawURL)
		assert.Equal(t, want, got)
		assert.NoError(t, err)
	})
	t.Run("URL with invalid escaped characters", func(t *testing.T) {
		rawURL := fmt.Sprintf(urlTemplate, "mygroup%**myproject")
		got, err := getIndexURL(rawURL)
		assert.Equal(t, "", got)
		assert.Error(t, err)
	})
}

func TestGetTagsFromUrl(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responseTags := TagsList{}
		w.Header().Set("Content-Type", "application/json")
		if !strings.Contains(r.URL.String(), "token") {
			w.Header().Set("Link", fmt.Sprintf("<https://%s%s?token=next-token>; rel=next", r.Host, r.URL.Path))
			responseTags.Tags = []string{"first"}
		} else {
			responseTags.Tags = []string{"second"}
		}
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(responseTags)
		if err != nil {
			t.Fatal(err)
		}
	}))

	client := NewClient(server.URL, Creds{InsecureSkipVerify: true}, true, "")

	tags, err := client.GetTags("mychart", true)
	assert.NoError(t, err)
	assert.Equal(t, tags.Tags[0], "first")
	assert.Equal(t, tags.Tags[1], "second")
}

func Test_getNextUrl(t *testing.T) {
	nextUrl := getNextUrl("")
	assert.Equal(t, nextUrl, "")

	nextUrl = getNextUrl("<https://my.repo.com/v2/chart/tags/list?token=123>; rel=next")
	assert.Equal(t, nextUrl, "https://my.repo.com/v2/chart/tags/list?token=123")
}

func Test_getTagsListURL(t *testing.T) {
	tagsListURL, err := getTagsListURL("account.dkr.ecr.eu-central-1.amazonaws.com", "dss")
	assert.Nil(t, err)
	assert.Equal(t, tagsListURL, "https://account.dkr.ecr.eu-central-1.amazonaws.com/v2/dss/tags/list")

	tagsListURL, err = getTagsListURL("http://account.dkr.ecr.eu-central-1.amazonaws.com", "dss")
	assert.Nil(t, err)
	assert.Equal(t, tagsListURL, "https://account.dkr.ecr.eu-central-1.amazonaws.com/v2/dss/tags/list")

	// with trailing /
	tagsListURL, err = getTagsListURL("https://account.dkr.ecr.eu-central-1.amazonaws.com/", "dss")
	assert.Nil(t, err)
	assert.Equal(t, tagsListURL, "https://account.dkr.ecr.eu-central-1.amazonaws.com/v2/dss/tags/list")
}
