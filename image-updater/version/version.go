package version

import (
	"fmt"
	"runtime"
)

var (
	version    = "9.9.99"
	buildDate  = "1970-01-01T00:00:00Z"
	gitCommit  = "unknown"
	binaryName = "argocd-image-updater"
)

func Version() string {
	version := fmt.Sprintf("v%s+%s", version, gitCommit[0:7])
	return version
}

func BinaryName() string {
	return binaryName
}

func Useragent() string {
	return fmt.Sprintf("%s: %s", BinaryName(), Version())
}

func GitCommit() string {
	return gitCommit
}

func BuildDate() string {
	return buildDate
}

func GoVersion() string {
	return runtime.Version()
}

func GoPlatform() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

func GoCompiler() string {
	return runtime.Compiler
}
