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

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/util/buffered_context"
	"github.com/argoproj/argo-cd/v2/util/cmp"
	"github.com/argoproj/argo-cd/v2/util/io/files"

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

func (s *Service) Init() error {
	workDir := common.GetCMPWorkDir()
	err := os.RemoveAll(workDir)
	if err != nil {
		return fmt.Errorf("error removing workdir %q: %s", workDir, err)
	}
	err = os.MkdirAll(workDir, 0700)
	if err != nil {
		return fmt.Errorf("error creating workdir %q: %s", workDir, err)
	}
	return nil
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
func (s *Service) GenerateManifest(stream apiclient.ConfigManagementPluginService_GenerateManifestServer) error {
	ctx, cancel := buffered_context.WithEarlierDeadline(stream.Context(), cmpTimeoutBuffer)
	defer cancel()
	workDir, err := files.CreateTempDir(common.GetCMPWorkDir())
	if err != nil {
		return fmt.Errorf("error creating temp dir: %s", err)
	}
	defer func() {
		if err := os.RemoveAll(workDir); err != nil {
			// we panic here as the workDir may contain sensitive information
			panic(fmt.Sprintf("error removing generate manifest workdir: %s", err))
		}
	}()

	metadata, err := cmp.ReceiveRepoStream(ctx, stream, workDir)
	if err != nil {
		return fmt.Errorf("generate manifest error receiving stream: %s", err)
	}

	appPath := filepath.Clean(filepath.Join(workDir, metadata.AppRelPath))
	if !strings.HasPrefix(appPath, workDir) {
		return fmt.Errorf("illegal appPath: out of workDir bound")
	}
	response, err := s.generateManifest(ctx, appPath, metadata.GetEnv())
	if err != nil {
		return fmt.Errorf("error generating manifests: %s", err)
	}
	err = stream.SendAndClose(response)
	if err != nil {
		return fmt.Errorf("error sending manifest response: %s", err)
	}
	return nil
}

// generateManifest runs generate command from plugin config file and returns generated manifest files
func (s *Service) generateManifest(ctx context.Context, appDir string, envEntries []*apiclient.EnvEntry) (*apiclient.ManifestResponse, error) {
	if deadline, ok := ctx.Deadline(); ok {
		log.Infof("Generating manifests with deadline %v from now", time.Until(deadline))
	} else {
		log.Info("Generating manifests with no request-level timeout")
	}

	config := s.initConstants.PluginConfig

	env := append(os.Environ(), environ(envEntries)...)
	if len(config.Spec.Init.Command) > 0 {
		_, err := runCommand(ctx, config.Spec.Init, appDir, env)
		if err != nil {
			return &apiclient.ManifestResponse{}, err
		}
	}

	out, err := runCommand(ctx, config.Spec.Generate, appDir, env)
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

// MatchRepository receives the application stream and checks whether
// its repository type is supported by the config management plugin
// server.
//The checks are implemented in the following order:
//   1. If spec.Discover.FileName is provided it finds for a name match in Applications files
//   2. If spec.Discover.Find.Glob is provided if finds for a glob match in Applications files
//   3. Otherwise it runs the spec.Discover.Find.Command
func (s *Service) MatchRepository(stream apiclient.ConfigManagementPluginService_MatchRepositoryServer) error {
	bufferedCtx, cancel := buffered_context.WithEarlierDeadline(stream.Context(), cmpTimeoutBuffer)
	defer cancel()

	workDir, err := files.CreateTempDir(common.GetCMPWorkDir())
	if err != nil {
		return fmt.Errorf("error creating match repository workdir: %s", err)
	}
	defer func() {
		if err := os.RemoveAll(workDir); err != nil {
			// we panic here as the workDir may contain sensitive information
			panic(fmt.Sprintf("error removing match repository workdir: %s", err))
		}
	}()

	_, err = cmp.ReceiveRepoStream(bufferedCtx, stream, workDir)
	if err != nil {
		return fmt.Errorf("match repository error receiving stream: %s", err)
	}

	isSupported, err := s.matchRepository(bufferedCtx, workDir)
	if err != nil {
		return fmt.Errorf("match repository error: %s", err)
	}
	repoResponse := &apiclient.RepositoryResponse{IsSupported: isSupported}

	err = stream.SendAndClose(repoResponse)
	if err != nil {
		return fmt.Errorf("error sending match repository response: %s", err)
	}
	return nil
}

func (s *Service) matchRepository(ctx context.Context, workdir string) (bool, error) {
	config := s.initConstants.PluginConfig
	if config.Spec.Discover.FileName != "" {
		log.Debugf("config.Spec.Discover.FileName is provided")
		pattern := filepath.Join(workdir, config.Spec.Discover.FileName)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			e := fmt.Errorf("error finding filename match for pattern %q: %s", pattern, err)
			log.Debug(e)
			return false, e
		}
		return len(matches) > 0, nil
	}

	if config.Spec.Discover.Find.Glob != "" {
		log.Debugf("config.Spec.Discover.Find.Glob is provided")
		pattern := filepath.Join(workdir, config.Spec.Discover.Find.Glob)
		// filepath.Glob doesn't have '**' support hence selecting third-party lib
		// https://github.com/golang/go/issues/11862
		matches, err := zglob.Glob(pattern)
		if err != nil {
			e := fmt.Errorf("error finding glob match for pattern %q: %s", pattern, err)
			log.Debug(e)
			return false, e
		}

		if len(matches) > 0 {
			return true, nil
		}
		return false, nil
	}

	log.Debugf("Going to try runCommand.")
	find, err := runCommand(ctx, config.Spec.Discover.Find.Command, workdir, os.Environ())
	if err != nil {
		return false, fmt.Errorf("error running find command: %s", err)
	}

	if find != "" {
		return true, nil
	}
	return false, nil
}
