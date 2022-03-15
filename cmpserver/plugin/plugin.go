package plugin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/argoproj/pkg/rand"

	"github.com/argoproj/argo-cd/v2/util/buffered_context"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/mattn/go-zglob"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v2/cmpserver/apiclient"
)

// cmpTimeoutBuffer is the amount of time before the request deadline to timeout server-side work. It makes sure there's
// enough time before the client times out to send a meaningful error message.
const cmpTimeoutBuffer = 100 * time.Millisecond

// Service implements ConfigManagementPluginService interface
type Service struct {
	initConstants CMPServerInitConstants
}

type CMPServerInitConstants struct {
	PluginConfig PluginConfig
}

// NewService returns a new instance of the ConfigManagementPluginService
func NewService(initConstants CMPServerInitConstants) *Service {
	return &Service{
		initConstants: initConstants,
	}
}

func runCommand(ctx context.Context, command Command, path string, env []string) (string, error) {
	if len(command.Command) == 0 {
		return "", fmt.Errorf("Command is empty")
	}
	cmd := exec.CommandContext(ctx, command.Command[0], append(command.Command[1:], command.Args...)...)

	cmd.Env = env
	cmd.Dir = path

	execId, err := rand.RandString(5)
	if err != nil {
		return "", err
	}
	logCtx := log.WithFields(log.Fields{"execID": execId})

	// log in a way we can copy-and-paste into a terminal
	args := strings.Join(cmd.Args, " ")
	logCtx.WithFields(log.Fields{"dir": cmd.Dir}).Info(args)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Make sure the command is killed immediately on timeout. https://stackoverflow.com/a/38133948/684776
	cmd.SysProcAttr = newSysProcAttr(true)

	start := time.Now()
	err = cmd.Start()
	if err != nil {
		return "", err
	}

	go func() {
		<-ctx.Done()
		// Kill by group ID to make sure child processes are killed. The - tells `kill` that it's a group ID.
		// Since we didn't set Pgid in SysProcAttr, the group ID is the same as the process ID. https://pkg.go.dev/syscall#SysProcAttr
		_ = sysCallKill(-cmd.Process.Pid)
	}()

	err = cmd.Wait()

	duration := time.Since(start)
	output := stdout.String()

	logCtx.WithFields(log.Fields{"duration": duration}).Debug(output)

	if err != nil {
		err := newCmdError(args, errors.New(err.Error()), strings.TrimSpace(stderr.String()))
		logCtx.Error(err.Error())
		return strings.TrimSuffix(output, "\n"), err
	}

	return strings.TrimSuffix(output, "\n"), nil
}

type CmdError struct {
	Args   string
	Stderr string
	Cause  error
}

func (ce *CmdError) Error() string {
	res := fmt.Sprintf("`%v` failed %v", ce.Args, ce.Cause)
	if ce.Stderr != "" {
		res = fmt.Sprintf("%s: %s", res, ce.Stderr)
	}
	return res
}

func newCmdError(args string, cause error, stderr string) *CmdError {
	return &CmdError{Args: args, Stderr: stderr, Cause: cause}
}

// Environ returns a list of environment variables in name=value format from a list of variables
func environ(envVars []*apiclient.EnvEntry) []string {
	var environ []string
	for _, item := range envVars {
		if item != nil && item.Name != "" && item.Value != "" {
			environ = append(environ, fmt.Sprintf("%s=%s", item.Name, item.Value))
		}
	}
	return environ
}

// GenerateManifest runs generate command from plugin config file and returns generated manifest files
func (s *Service) GenerateManifest(ctx context.Context, q *apiclient.ManifestRequest) (*apiclient.ManifestResponse, error) {
	bufferedCtx, cancel := buffered_context.WithEarlierDeadline(ctx, cmpTimeoutBuffer)
	defer cancel()

	if deadline, ok := bufferedCtx.Deadline(); ok {
		log.Infof("Generating manifests with deadline %v from now", time.Until(deadline))
	} else {
		log.Info("Generating manifests with no request-level timeout")
	}

	config := s.initConstants.PluginConfig

	env := append(os.Environ(), environ(q.Env)...)
	if len(config.Spec.Init.Command) > 0 {
		_, err := runCommand(bufferedCtx, config.Spec.Init, q.AppPath, env)
		if err != nil {
			return &apiclient.ManifestResponse{}, err
		}
	}

	out, err := runCommand(bufferedCtx, config.Spec.Generate, q.AppPath, env)
	if err != nil {
		return &apiclient.ManifestResponse{}, err
	}

	manifests, err := kube.SplitYAMLToString([]byte(out))
	if err != nil {
		return &apiclient.ManifestResponse{}, err
	}

	return &apiclient.ManifestResponse{
		Manifests: manifests,
	}, err
}

// MatchRepository checks whether the application repository type is supported by config management plugin server
func (s *Service) MatchRepository(ctx context.Context, q *apiclient.RepositoryRequest) (*apiclient.RepositoryResponse, error) {
	bufferedCtx, cancel := buffered_context.WithEarlierDeadline(ctx, cmpTimeoutBuffer)
	defer cancel()

	var repoResponse apiclient.RepositoryResponse
	config := s.initConstants.PluginConfig
	if config.Spec.Discover.FileName != "" {
		log.Debugf("config.Spec.Discover.FileName is provided")
		pattern := strings.TrimSuffix(q.Path, "/") + "/" + strings.TrimPrefix(config.Spec.Discover.FileName, "/")
		matches, err := filepath.Glob(pattern)
		if err != nil || len(matches) == 0 {
			log.Debugf("Could not find match for pattern %s. Error is %v.", pattern, err)
			return &repoResponse, err
		} else if len(matches) > 0 {
			repoResponse.IsSupported = true
			return &repoResponse, nil
		}
	}

	if config.Spec.Discover.Find.Glob != "" {
		log.Debugf("config.Spec.Discover.Find.Glob is provided")
		pattern := strings.TrimSuffix(q.Path, "/") + "/" + strings.TrimPrefix(config.Spec.Discover.Find.Glob, "/")
		// filepath.Glob doesn't have '**' support hence selecting third-party lib
		// https://github.com/golang/go/issues/11862
		matches, err := zglob.Glob(pattern)
		if err != nil || len(matches) == 0 {
			log.Debugf("Could not find match for pattern %s. Error is %v.", pattern, err)
			return &repoResponse, err
		} else if len(matches) > 0 {
			repoResponse.IsSupported = true
			return &repoResponse, nil
		}
	}

	log.Debugf("Going to try runCommand.")
	find, err := runCommand(bufferedCtx, config.Spec.Discover.Find.Command, q.Path, os.Environ())
	if err != nil {
		return &repoResponse, err
	}

	var isSupported bool
	if find != "" {
		isSupported = true
	}
	return &apiclient.RepositoryResponse{
		IsSupported: isSupported,
	}, nil
}

// GetPluginConfig returns plugin config
func (s *Service) GetPluginConfig(ctx context.Context, q *apiclient.ConfigRequest) (*apiclient.ConfigResponse, error) {
	config := s.initConstants.PluginConfig
	return &apiclient.ConfigResponse{
		AllowConcurrency: config.Spec.AllowConcurrency,
		LockRepo:         config.Spec.LockRepo,
	}, nil
}
