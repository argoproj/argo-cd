package version

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/argoproj/gitops-engine/pkg/utils/tracing"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/go-jsonnet"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apiclient/version"
	"github.com/argoproj/argo-cd/util/helm"
	ksutil "github.com/argoproj/argo-cd/util/ksonnet"
	"github.com/argoproj/argo-cd/util/kustomize"
	"github.com/argoproj/argo-cd/util/log"
)

type Server struct {
	ksonnetVersion   string
	kustomizeVersion string
	helmVersion      string
	kubectlVersion   string
	jsonnetVersion   string
}

func getVersion() (string, error) {
	span := tracing.NewLoggingTracer(log.NewLogrusLogger(logrus.New())).StartSpan("Version")
	defer span.Finish()
	cmd := exec.Command("kubectl", "version", "--client")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("could not get kubectl version: %s", err)
	}

	re := regexp.MustCompile(`GitVersion:"([a-zA-Z0-9\.\-]+)"`)
	matches := re.FindStringSubmatch(string(out))
	if len(matches) != 2 {
		return "", errors.New("could not get kubectl version")
	}
	version := matches[1]
	if version[0] != 'v' {
		version = "v" + version
	}
	return strings.TrimSpace(version), nil
}

// Version returns the version of the API server
func (s *Server) Version(context.Context, *empty.Empty) (*version.VersionMessage, error) {
	vers := common.GetVersion()
	if s.ksonnetVersion == "" {
		ksonnetVersion, err := ksutil.Version()
		if err == nil {
			s.ksonnetVersion = ksonnetVersion
		} else {
			s.ksonnetVersion = err.Error()
		}
	}
	if s.kustomizeVersion == "" {
		kustomizeVersion, err := kustomize.Version(true)
		if err == nil {
			s.kustomizeVersion = kustomizeVersion
		} else {
			s.kustomizeVersion = err.Error()
		}
	}
	if s.helmVersion == "" {
		helmVersion, err := helm.Version(true)
		if err == nil {
			s.helmVersion = helmVersion
		} else {
			s.helmVersion = err.Error()
		}
	}
	if s.kubectlVersion == "" {
		kubectlVersion, err := getVersion()
		if err == nil {
			s.kubectlVersion = kubectlVersion
		} else {
			s.kubectlVersion = err.Error()
		}
	}
	s.jsonnetVersion = jsonnet.Version()
	return &version.VersionMessage{
		Version:          vers.Version,
		BuildDate:        vers.BuildDate,
		GitCommit:        vers.GitCommit,
		GitTag:           vers.GitTag,
		GitTreeState:     vers.GitTreeState,
		GoVersion:        vers.GoVersion,
		Compiler:         vers.Compiler,
		Platform:         vers.Platform,
		KsonnetVersion:   s.ksonnetVersion,
		KustomizeVersion: s.kustomizeVersion,
		HelmVersion:      s.helmVersion,
		KubectlVersion:   s.kubectlVersion,
		JsonnetVersion:   s.jsonnetVersion,
	}, nil
}

// AuthFuncOverride allows the version to be returned without auth
func (s *Server) AuthFuncOverride(ctx context.Context, fullMethodName string) (context.Context, error) {
	return ctx, nil
}
