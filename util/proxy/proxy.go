package proxy

import (
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
)

// UpsertEnv removes the existing proxy env variables and adds the custom proxy variables
func UpsertEnv(cmd *exec.Cmd, proxy string) []string {
	envs := []string{}
	if proxy == "" {
		return cmd.Env
	}
	// remove the existing proxy env variable if present
	for i, env := range cmd.Env {
		proxyEnv := strings.ToLower(env)
		if strings.HasPrefix(proxyEnv, "http_proxy") || strings.HasPrefix(proxyEnv, "https_proxy") {
			continue
		}
		envs = append(envs, cmd.Env[i])
	}
	return append(envs, httpProxy(proxy), httpsProxy(proxy))
}

// GetCallback returns the proxy callback function
func GetCallback(proxy string) func(*http.Request) (*url.URL, error) {
	if proxy != "" {
		return func(r *http.Request) (*url.URL, error) {
			return url.Parse(proxy)
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
