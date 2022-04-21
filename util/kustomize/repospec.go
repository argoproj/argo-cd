// The following file was copied from https://github.com/kubernetes-sigs/kustomize/blob/master/api/internal/git/repospec.go
// and modified to expose the ParseGitUrl function
//
// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0
package kustomize

import (
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	gitSuffix    = ".git"
	gitDelimiter = "_git/"
)

// From strings like git@github.com:someOrg/someRepo.git or
// https://github.com/someOrg/someRepo?ref=someHash, extract
// the parts.
func parseGitURL(n string) (
	host string, orgRepo string, path string, gitRef string, gitSubmodules bool, gitSuff string, gitTimeout time.Duration) {
	if strings.Contains(n, gitDelimiter) {
		index := strings.Index(n, gitDelimiter)
		// Adding _git/ to host
		host = normalizeGitHostSpec(n[:index+len(gitDelimiter)])
		orgRepo = strings.Split(strings.Split(n[index+len(gitDelimiter):], "/")[0], "?")[0]
		path, gitRef, gitTimeout, gitSubmodules = peelQuery(n[index+len(gitDelimiter)+len(orgRepo):])
		return
	}
	host, n = parseHostSpec(n)
	gitSuff = gitSuffix
	if strings.Contains(n, gitSuffix) {
		index := strings.Index(n, gitSuffix)
		orgRepo = n[0:index]
		n = n[index+len(gitSuffix):]
		if len(n) > 0 && n[0] == '/' {
			n = n[1:]
		}
		path, gitRef, gitTimeout, gitSubmodules = peelQuery(n)
		return
	}

	i := strings.Index(n, "/")
	if i < 1 {
		path, gitRef, gitTimeout, gitSubmodules = peelQuery(n)
		return
	}
	j := strings.Index(n[i+1:], "/")
	if j >= 0 {
		j += i + 1
		orgRepo = n[:j]
		path, gitRef, gitTimeout, gitSubmodules = peelQuery(n[j+1:])
		return
	}
	path = ""
	orgRepo, gitRef, gitTimeout, gitSubmodules = peelQuery(n)
	return host, orgRepo, path, gitRef, gitSubmodules, gitSuff, gitTimeout
}

// Clone git submodules by default.
const defaultSubmodules = true

// Arbitrary, but non-infinite, timeout for running commands.
const defaultTimeout = 27 * time.Second

func peelQuery(arg string) (string, string, time.Duration, bool) {
	// Parse the given arg into a URL. In the event of a parse failure, return
	// our defaults.
	parsed, err := url.Parse(arg)
	if err != nil {
		return arg, "", defaultTimeout, defaultSubmodules
	}
	values := parsed.Query()

	// ref is the desired git ref to target. Can be specified by in a git URL
	// with ?ref=<string> or ?version=<string>, although ref takes precedence.
	ref := values.Get("version")
	if queryValue := values.Get("ref"); queryValue != "" {
		ref = queryValue
	}

	// depth is the desired git exec timeout. Can be specified by in a git URL
	// with ?timeout=<duration>.
	duration := defaultTimeout
	if queryValue := values.Get("timeout"); queryValue != "" {
		// Attempt to first parse as a number of integer seconds (like "61"),
		// and then attempt to parse as a suffixed duration (like "61s").
		if intValue, err := strconv.Atoi(queryValue); err == nil && intValue > 0 {
			duration = time.Duration(intValue) * time.Second
		} else if durationValue, err := time.ParseDuration(queryValue); err == nil && durationValue > 0 {
			duration = durationValue
		}
	}

	// submodules indicates if git submodule cloning is desired. Can be
	// specified by in a git URL with ?submodules=<bool>.
	submodules := defaultSubmodules
	if queryValue := values.Get("submodules"); queryValue != "" {
		if boolValue, err := strconv.ParseBool(queryValue); err == nil {
			submodules = boolValue
		}
	}

	return parsed.Path, ref, duration, submodules
}

func parseHostSpec(n string) (string, string) {
	var host string
	// Start accumulating the host part.
	for _, p := range []string{
		// Order matters here.
		"git::", "gh:", "ssh://", "https://", "http://",
		"git@", "github.com:", "github.com/"} {
		if len(p) < len(n) && strings.ToLower(n[:len(p)]) == p {
			n = n[len(p):]
			host += p
		}
	}
	if host == "git@" {
		i := strings.Index(n, "/")
		if i > -1 {
			host += n[:i+1]
			n = n[i+1:]
		} else {
			i = strings.Index(n, ":")
			if i > -1 {
				host += n[:i+1]
				n = n[i+1:]
			}
		}
		return host, n
	}

	// If host is a http(s) or ssh URL, grab the domain part.
	for _, p := range []string{
		"ssh://", "https://", "http://"} {
		if strings.HasSuffix(host, p) {
			i := strings.Index(n, "/")
			if i > -1 {
				host += n[0 : i+1]
				n = n[i+1:]
			}
			break
		}
	}

	return normalizeGitHostSpec(host), n
}

func normalizeGitHostSpec(host string) string {
	s := strings.ToLower(host)
	if strings.Contains(s, "github.com") {
		if strings.Contains(s, "git@") || strings.Contains(s, "ssh:") {
			host = "git@github.com:"
		} else {
			host = "https://github.com/"
		}
	}
	if strings.HasPrefix(s, "git::") {
		host = strings.TrimPrefix(s, "git::")
	}
	return host
}

// The format of Azure repo URL is documented
// https://docs.microsoft.com/en-us/azure/devops/repos/git/clone?view=vsts&tabs=visual-studio#clone_url
func isAzureHost(host string) bool {
	return strings.Contains(host, "dev.azure.com") ||
		strings.Contains(host, "visualstudio.com")
}

// The format of AWS repo URL is documented
// https://docs.aws.amazon.com/codecommit/latest/userguide/regions.html
func isAWSHost(host string) bool {
	return strings.Contains(host, "amazonaws.com")
}
