package proxy

import (
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
)

// AddEnvIfAbsent adds the proxy URL as an env variable if absent
func AddEnvIfAbsent(cmd *exec.Cmd, proxy string) []string {
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
	if strings.HasPrefix(proxy, "https://") {
		envs = append(envs, httpsProxy(proxy))
	} else {
		envs = append(envs, httpProxy(proxy))
	}
	return envs
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
