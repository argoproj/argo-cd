package common

import (
	"fmt"
	"runtime"
)

// Version information set by link flags during build. We fall back to these sane
// default values when we build outside the Makefile context (e.g. go run, go build, or go test).
var (
	version        = "99.99.99"             // value from VERSION file
	buildDate      = "1970-01-01T00:00:00Z" // output from `date -u +'%Y-%m-%dT%H:%M:%SZ'`
	gitCommit      = ""                     // output from `git rev-parse HEAD`
	gitTag         = ""                     // output from `git describe --exact-match --tags HEAD` (if clean tree state)
	gitTreeState   = ""                     // determined from `git status --porcelain`. either 'clean' or 'dirty'
	kubectlVersion = ""                     // determined from go.mod file
)

// Version contains Argo version information
type Version struct {
	Version        string
	BuildDate      string
	GitCommit      string
	GitTag         string
	GitTreeState   string
	GoVersion      string
	Compiler       string
	Platform       string
	KubectlVersion string
}

func (v Version) String() string {
	return v.Version
}

// GetVersion returns the version information
func GetVersion() Version {
	var versionStr string

	if gitCommit != "" && gitTag != "" && gitTreeState == "clean" {
		// if we have a clean tree state and the current commit is tagged,
		// this is an official release.
		versionStr = gitTag
	} else {
		// otherwise formulate a version string based on as much metadata
		// information we have available.
		versionStr = "v" + version
		if len(gitCommit) >= 7 {
			versionStr += "+" + gitCommit[0:7]
			if gitTreeState != "clean" {
				versionStr += ".dirty"
			}
		} else {
			versionStr += "+unknown"
		}
	}
	return Version{
		Version:        versionStr,
		BuildDate:      buildDate,
		GitCommit:      gitCommit,
		GitTag:         gitTag,
		GitTreeState:   gitTreeState,
		GoVersion:      runtime.Version(),
		Compiler:       runtime.Compiler,
		Platform:       fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		KubectlVersion: kubectlVersion,
	}
}
