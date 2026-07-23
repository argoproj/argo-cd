package apiclient

import (
	"fmt"
	"net/url"
	"strings"
)

// ValidateServerAddress rejects URL schemes and path-bearing addresses that the API client cannot dial directly.
func ValidateServerAddress(server string) error {
	lowerServer := strings.ToLower(server)
	if strings.HasPrefix(lowerServer, "http://") || strings.HasPrefix(lowerServer, "https://") {
		parsed, err := url.Parse(server)
		if err != nil || parsed.Host == "" {
			return fmt.Errorf("server address %q must be a host and optional port without http:// or https://", server)
		}

		flags := make([]string, 0, 3)
		if strings.EqualFold(parsed.Scheme, "http") {
			flags = append(flags, "--plaintext")
		}
		rootPath := strings.TrimRight(parsed.Path, "/")
		if rootPath != "" {
			flags = append(flags, fmt.Sprintf("--grpc-web-root-path %q", rootPath))
		}

		invalidParts := "a URL scheme"
		if rootPath != "" {
			invalidParts = "a URL scheme or path"
		}
		guidance := fmt.Sprintf("use %q", parsed.Host)
		if len(flags) > 0 {
			guidance += " with " + strings.Join(flags, " ")
		}
		return fmt.Errorf("server address %q must not include %s; %s instead", server, invalidParts, guidance)
	}
	if strings.Contains(server, "://") {
		return fmt.Errorf("server address %q must be a host and optional port without a URL scheme", server)
	}

	if host, path, found := strings.Cut(server, "/"); found {
		rootPath := strings.TrimRight("/"+strings.TrimLeft(path, "/"), "/")
		if host == "" {
			return fmt.Errorf("server address %q must be a host and optional port", server)
		}
		if rootPath == "" {
			return fmt.Errorf("server address %q must not include a path; use %q instead", server, host)
		}
		return fmt.Errorf("server address %q must not include a path; use %q with --grpc-web-root-path %q instead", server, host, rootPath)
	}

	return nil
}
