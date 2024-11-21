package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddProxyEnvIfAbsent(t *testing.T) {
	t.Run("Existing proxy env variables", func(t *testing.T) {
		proxy := "https://proxy:5000"
		noProxy := ".argoproj.io"
		cmd := exec.Command("test")
		cmd.Env = []string{`http_proxy="https_proxy=https://env-proxy:8888"`, "key=val", "no_proxy=.argoproj.io"}
		got := UpsertEnv(cmd, proxy, noProxy)
		assert.EqualValues(t, []string{"key=val", httpProxy(proxy), httpsProxy(proxy), noProxyVar(noProxy)}, got)
	})
	t.Run("proxy env variables not found", func(t *testing.T) {
		proxy := "http://proxy:5000"
		noProxy := ".argoproj.io"
		cmd := exec.Command("test")
		cmd.Env = []string{"key=val"}
		got := UpsertEnv(cmd, proxy, noProxy)
		assert.EqualValues(t, []string{"key=val", httpProxy(proxy), httpsProxy(proxy), noProxyVar(noProxy)}, got)
	})
}

func TestGetCallBack(t *testing.T) {
	t.Run("custom proxy present", func(t *testing.T) {
		proxy := "http://proxy:8888"
		url, err := GetCallback(proxy, "")(nil)
		require.NoError(t, err)
		assert.Equal(t, proxy, url.String())
	})
	t.Run("custom proxy present, noProxy filteres request", func(t *testing.T) {
		proxy := "http://proxy:8888"
		noProxy := "argoproj.io"
		url, err := GetCallback(proxy, noProxy)(&http.Request{URL: &url.URL{Host: "argoproj.io"}})
		require.NoError(t, err)
		assert.Nil(t, url) // proxy object being nil indicates that no proxy should be used for this request
	})
	t.Run("custom proxy absent", func(t *testing.T) {
		proxyEnv := "http://proxy:8888"
		t.Setenv("http_proxy", "http://proxy:8888")
		url, err := GetCallback("", "")(httptest.NewRequest(http.MethodGet, proxyEnv, nil))
		require.NoError(t, err)
		assert.Equal(t, proxyEnv, url.String())
	})
}
