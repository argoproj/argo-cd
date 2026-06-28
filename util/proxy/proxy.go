package proxy

import (
	"net/http"
	"net/url"
	"os/exec"
	"strings"

	"golang.org/x/net/http/httpproxy"
)

// DefaultProxyCallback is the default proxy callback function that reads from environment variables. http.ProxyFromEnvironment
// is cached on first call, so we can't use it for tests. When writing a test that uses t.Setenv for some proxy env var,
// call UseTestingProxyCallback.
var DefaultProxyCallback = http.ProxyFromEnvironment

// UseTestingProxyCallback sets the DefaultProxyCallback to use httpproxy.FromEnvironment. This is useful for tests that
// use t.Setenv to set proxy env variables.
func UseTestingProxyCallback() {
	DefaultProxyCallback = func(r *http.Request) (*url.URL, error) {
		return httpproxy.FromEnvironment().ProxyFunc()(r.URL)
	}
}

// UpsertEnv removes the existing proxy env variables and adds the custom proxy variables
func UpsertEnv(cmd *exec.Cmd, proxy string, noProxy string) []string {
	envs := []string{}
	if proxy == "" {
		return cmd.Env
	}
	// remove the existing proxy env variable if present
	for i, env := range cmd.Env {
		proxyEnv := strings.ToLower(env)
		if strings.HasPrefix(proxyEnv, "http_proxy") || strings.HasPrefix(proxyEnv, "https_proxy") || strings.HasPrefix(proxyEnv, "no_proxy") {
			continue
		}
		envs = append(envs, cmd.Env[i])
	}
	return append(envs, httpProxy(proxy), httpsProxy(proxy), noProxyVar(noProxy))
}

// GetCallback returns the proxy callback function
func GetCallback(proxy string, noProxy string) func(*http.Request) (*url.URL, error) {
	if proxy != "" {
		c := httpproxy.Config{
			HTTPProxy:  proxy,
			HTTPSProxy: proxy,
			NoProxy:    noProxy,
		}
		return func(r *http.Request) (*url.URL, error) {
			if r != nil {
				return c.ProxyFunc()(r.URL)
			}
			return url.Parse(c.HTTPProxy)
		}
	}
	// read proxy from env variable if custom proxy is missing
	return DefaultProxyCallback
}

func httpProxy(url string) string {
	return "http_proxy=" + url
}

func httpsProxy(url string) string {
	return "https_proxy=" + url
}

func noProxyVar(noProxy string) string {
	return "no_proxy=" + noProxy
}
