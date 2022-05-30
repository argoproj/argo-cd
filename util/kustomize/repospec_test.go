// The following file was copied from https://github.com/kubernetes-sigs/kustomize/blob/master/api/internal/git/repospec.go
// and modified to expose the ParseGitUrl function
//
// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package kustomize

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	refQuery = "?ref="
)

var orgRepos = []string{"someOrg/someRepo", "kubernetes/website"}

var pathNames = []string{"README.md", "foo/krusty.txt", ""}

var hrefArgs = []string{"someBranch", "master", "v0.1.0", ""}

var hostNamesRawAndNormalized = [][]string{
	{"gh:", "gh:"},
	{"GH:", "gh:"},
	{"gitHub.com/", "https://github.com/"},
	{"github.com:", "https://github.com/"},
	{"http://github.com/", "https://github.com/"},
	{"https://github.com/", "https://github.com/"},
	{"hTTps://github.com/", "https://github.com/"},
	{"https://git-codecommit.us-east-2.amazonaws.com/", "https://git-codecommit.us-east-2.amazonaws.com/"},
	{"https://fabrikops2.visualstudio.com/", "https://fabrikops2.visualstudio.com/"},
	{"ssh://git.example.com:7999/", "ssh://git.example.com:7999/"},
	{"git::https://gitlab.com/", "https://gitlab.com/"},
	{"git::http://git.example.com/", "http://git.example.com/"},
	{"git::https://git.example.com/", "https://git.example.com/"},
	{"git@github.com:", "git@github.com:"},
	{"git@github.com/", "git@github.com:"},
}

func makeURL(hostFmt, orgRepo, path, href string) string {
	if len(path) > 0 {
		orgRepo = filepath.Join(orgRepo, path)
	}
	url := hostFmt + orgRepo
	if href != "" {
		url += refQuery + href
	}
	return url
}

func TestNewRepoSpecFromUrl(t *testing.T) {
	var bad [][]string
	for _, tuple := range hostNamesRawAndNormalized {
		hostRaw := tuple[0]
		hostSpec := tuple[1]
		for _, orgRepo := range orgRepos {
			for _, pathName := range pathNames {
				for _, hrefArg := range hrefArgs {
					uri := makeURL(hostRaw, orgRepo, pathName, hrefArg)
					host, org, path, ref := parseGitURL(uri)
					if host != hostSpec {
						bad = append(bad, []string{"host", uri, host, hostSpec})
					}
					if org != orgRepo {
						bad = append(bad, []string{"orgRepo", uri, org, orgRepo})
					}
					if path != pathName {
						bad = append(bad, []string{"path", uri, path, pathName})
					}
					if ref != hrefArg {
						bad = append(bad, []string{"ref", uri, ref, hrefArg})
					}
				}
			}
		}
	}
	if len(bad) > 0 {
		for _, tuple := range bad {
			fmt.Printf("\n"+
				"     from uri: %s\n"+
				"  actual %4s: %s\n"+
				"expected %4s: %s\n",
				tuple[1], tuple[0], tuple[2], tuple[0], tuple[3])
		}
		t.Fail()
	}
}

func TestIsAzureHost(t *testing.T) {
	testcases := []struct {
		input  string
		expect bool
	}{
		{
			input:  "https://git-codecommit.us-east-2.amazonaws.com",
			expect: false,
		},
		{
			input:  "ssh://git-codecommit.us-east-2.amazonaws.com",
			expect: false,
		},
		{
			input:  "https://fabrikops2.visualstudio.com/",
			expect: true,
		},
		{
			input:  "https://dev.azure.com/myorg/myproject/",
			expect: true,
		},
	}
	for _, testcase := range testcases {
		actual := isAzureHost(testcase.input)
		if actual != testcase.expect {
			t.Errorf("IsAzureHost: expected %v, but got %v on %s", testcase.expect, actual, testcase.input)
		}
	}
}

func TestPeelQuery(t *testing.T) {
	testcases := map[string]struct {
		input string
		path  string
		ref   string
	}{
		"t1": {
			// All empty.
			input: "somerepos",
			path:  "somerepos",
			ref:   "",
		},
		"t2": {
			input: "somerepos?ref=v1.0.0",
			path:  "somerepos",
			ref:   "v1.0.0",
		},
		"t3": {
			input: "somerepos?version=master",
			path:  "somerepos",
			ref:   "master",
		},
		"t4": {
			// A ref value takes precedence over a version value.
			input: "somerepos?version=master&ref=v1.0.0",
			path:  "somerepos",
			ref:   "v1.0.0",
		},
		"t5": {
			// Empty submodules value uses default.
			input: "somerepos?version=master&submodules=",
			path:  "somerepos",
			ref:   "master",
		},
		"t6": {
			// Malformed submodules value uses default.
			input: "somerepos?version=master&submodules=maybe",
			path:  "somerepos",
			ref:   "master",
		},
		"t7": {
			input: "somerepos?version=master&submodules=true",
			path:  "somerepos",
			ref:   "master",
		},
		"t8": {
			input: "somerepos?version=master&submodules=false",
			path:  "somerepos",
			ref:   "master",
		},
		"t9": {
			// Empty timeout value uses default.
			input: "somerepos?version=master&timeout=",
			path:  "somerepos",
			ref:   "master",
		},
		"t10": {
			// Malformed timeout value uses default.
			input: "somerepos?version=master&timeout=jiffy",
			path:  "somerepos",
			ref:   "master",
		},
		"t11": {
			// Zero timeout value uses default.
			input: "somerepos?version=master&timeout=0",
			path:  "somerepos",
			ref:   "master",
		},
		"t12": {
			input: "somerepos?version=master&timeout=0s",
			path:  "somerepos",
			ref:   "master",
		},
		"t13": {
			input: "somerepos?version=master&timeout=61",
			path:  "somerepos",
			ref:   "master",
		},
		"t14": {
			input: "somerepos?version=master&timeout=1m1s",
			path:  "somerepos",
			ref:   "master",
		},
		"t15": {
			input: "somerepos?version=master&submodules=false&timeout=1m1s",
			path:  "somerepos",
			ref:   "master",
		},
	}
	for tn, tc := range testcases {
		t.Run(tn, func(t *testing.T) {
			path, ref := peelQuery(tc.input)
			assert.Equal(t, tc.path, path, "path mismatch")
			assert.Equal(t, tc.ref, ref, "ref mismatch")
		})
	}
}

func TestIsAWSHost(t *testing.T) {
	testcases := []struct {
		input  string
		expect bool
	}{
		{
			input:  "https://git-codecommit.us-east-2.amazonaws.com",
			expect: true,
		},
		{
			input:  "ssh://git-codecommit.us-east-2.amazonaws.com",
			expect: true,
		},
		{
			input:  "git@github.com:",
			expect: false,
		},
		{
			input:  "http://github.com/",
			expect: false,
		},
	}
	for _, testcase := range testcases {
		actual := isAWSHost(testcase.input)
		if actual != testcase.expect {
			t.Errorf("IsAWSHost: expected %v, but got %v on %s", testcase.expect, actual, testcase.input)
		}
	}
}
