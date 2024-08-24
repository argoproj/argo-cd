package proxy

import (
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strings"

	"golang.org/x/net/http/httpproxy"
)

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
	return http.ProxyFromEnvironment
}

func httpProxy(url string) string {
	return fmt.Sprintf("http_proxy=%s", url)
}

func httpsProxy(url string) string {
	return fmt.Sprintf("https_proxy=%s", url)
}

func noProxyVar(noProxy string) string {
	return fmt.Sprintf("no_proxy=%s", noProxy)
}
