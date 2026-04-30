package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"github.com/argoproj/argo-cd/v3/cmpserver/apiclient"
	"github.com/argoproj/argo-cd/v3/common"
	repoclient "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/util/buffered_context"
	"github.com/argoproj/argo-cd/v3/util/cmp"
	argoexec "github.com/argoproj/argo-cd/v3/util/exec"
	"github.com/argoproj/argo-cd/v3/util/io/files"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/mattn/go-zglob"
	log "github.com/sirupsen/logrus"
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

func (s *Service) Init(workDir string) error {
	err := os.RemoveAll(workDir)
	if err != nil {
		return fmt.Errorf("error removing workdir %q: %w", workDir, err)
	}
	err = os.MkdirAll(workDir, 0o700)
	if err != nil {
		return fmt.Errorf("error creating workdir %q: %w", workDir, err)
	}
	return nil
}

func runCommand(ctx context.Context, command Command, path string, env []string) (string, error) {
	if len(command.Command) == 0 {
		return "", errors.New("Command is empty")
	}
	cmd := exec.CommandContext(ctx, command.Command[0], append(command.Command[1:], command.Args...)...)

	cmd.Env = env
	cmd.Dir = path

	// Make sure the command is killed immediately on timeout by setting process group
	// This maintains backward compatibility with the original plugin behavior
	// https://stackoverflow.com/a/38133948/684776
	cmd.SysProcAttr = newSysProcAttr(true)

	// The argoexec package handles both context cancellation and its own timeout mechanisms
	// Context timeout takes precedence when it exists, ARGOCD_EXEC_TIMEOUT is used as fallback
	var timeout time.Duration
	if deadline, ok := ctx.Deadline(); ok {
		contextTimeout := time.Until(deadline)
		// If context is already expired, return immediately
		if contextTimeout <= 0 {
			return "", ctx.Err()
		}
		// Use context timeout for argoexec - this ensures consistency with Git/Helm/Kustomize operations
		timeout = contextTimeout
	}
	// If no context timeout, argoexec will use ARGOCD_EXEC_TIMEOUT (by setting timeout to 0)

	cmdOpts := argoexec.CmdOpts{
		// Use context timeout if present, otherwise argoexec will use ARGOCD_EXEC_TIMEOUT
		Timeout: timeout,
		// CaptureStderr is set to true to include stderr in the output for better error reporting
		CaptureStderr: true,
		// TimeoutBehavior maintains backward compatibility: send SIGTERM first, then wait for cleanup
		TimeoutBehavior: argoexec.TimeoutBehavior{
			Signal: syscall.SIGTERM, // Send SIGTERM first to allow cleanup
		},
	}

	return argoexec.RunCommandExt(cmd, cmdOpts)
}

// Environ returns a list of environment variables in name=value format from a list of variables
func environ(envVars []*apiclient.EnvEntry) []string {
	var environ []string
	for _, item := range envVars {
		if item != nil && item.Name != "" {
			environ = append(environ, fmt.Sprintf("%s=%s", item.Name, item.Value))
		}
	}
	return environ
}

// getTempDirMustCleanup creates a temporary directory and returns a cleanup function.
func getTempDirMustCleanup(baseDir string) (workDir string, cleanup func(), err error) {
	workDir, err = files.CreateTempDir(baseDir)
	if err != nil {
		return "", nil, fmt.Errorf("error creating temp dir: %w", err)
	}
	cleanup = func() {
		if err := os.RemoveAll(workDir); err != nil {
			log.WithFields(map[string]any{
				common.SecurityField:    common.SecurityHigh,
				common.SecurityCWEField: common.SecurityCWEIncompleteCleanup,
			}).Errorf("Failed to clean up temp directory: %s", err)
		}
	}
	return workDir, cleanup, nil
}

type Stream interface {
	Recv() (*apiclient.AppStreamRequest, error)
	Context() context.Context
}

type GenerateManifestStream interface {
	Stream
	SendAndClose(response *apiclient.ManifestResponse) error
}

// GenerateManifest runs generate command from plugin config file and returns generated manifest files
func (s *Service) GenerateManifest(stream apiclient.ConfigManagementPluginService_GenerateManifestServer) error {
	return s.generateManifestGeneric(stream)
}

func (s *Service) generateManifestGeneric(stream GenerateManifestStream) error {
	ctx, cancel := buffered_context.WithEarlierDeadline(stream.Context(), cmpTimeoutBuffer)
	defer cancel()
	workDir, cleanup, err := getTempDirMustCleanup(common.GetCMPWorkDir())
	if err != nil {
		return fmt.Errorf("error creating workdir for manifest generation: %w", err)
	}
	defer cleanup()

	metadata, err := cmp.ReceiveRepoStream(ctx, stream, workDir, s.initConstants.PluginConfig.Spec.PreserveFileMode)
	if err != nil {
		return fmt.Errorf("generate manifest error receiving stream: %w", err)
	}

	appPath := filepath.Clean(filepath.Join(workDir, metadata.AppRelPath))
	if !strings.HasPrefix(appPath, workDir) {
		return errors.New("illegal appPath: out of workDir bound")
	}
	response, err := s.generateManifest(ctx, appPath, metadata.GetEnv())
	if err != nil {
		return fmt.Errorf("error generating manifests: %w", err)
	}

	log.Tracef("Generated manifests result: %s", response.Manifests)

	err = stream.SendAndClose(response)
	if err != nil {
		return fmt.Errorf("error sending manifest response: %w", err)
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
		sanitizedManifests := manifests
		if len(sanitizedManifests) > 1000 {
			sanitizedManifests = manifests[:1000]
		}
		log.Debugf("Failed to split generated manifests. Beginning of generated manifests: %q", sanitizedManifests)
		return &apiclient.ManifestResponse{}, err
	}

	return &apiclient.ManifestResponse{
		Manifests: manifests,
	}, err
}

type MatchRepositoryStream interface {
	Stream
	SendAndClose(response *apiclient.RepositoryResponse) error
}

// MatchRepository receives the application stream and checks whether
// its repository type is supported by the config management plugin
// server.
// The checks are implemented in the following order:
//  1. If spec.Discover.FileName is provided it finds for a name match in Applications files
//  2. If spec.Discover.Find.Glob is provided if finds for a glob match in Applications files
//  3. Otherwise it runs the spec.Discover.Find.Command
func (s *Service) MatchRepository(stream apiclient.ConfigManagementPluginService_MatchRepositoryServer) error {
	return s.matchRepositoryGeneric(stream)
}

func (s *Service) matchRepositoryGeneric(stream MatchRepositoryStream) error {
	bufferedCtx, cancel := buffered_context.WithEarlierDeadline(stream.Context(), cmpTimeoutBuffer)
	defer cancel()

	workDir, cleanup, err := getTempDirMustCleanup(common.GetCMPWorkDir())
	if err != nil {
		return fmt.Errorf("error creating workdir for repository matching: %w", err)
	}
	defer cleanup()

	metadata, err := cmp.ReceiveRepoStream(bufferedCtx, stream, workDir, s.initConstants.PluginConfig.Spec.PreserveFileMode)
	if err != nil {
		return fmt.Errorf("match repository error receiving stream: %w", err)
	}

	isSupported, isDiscoveryEnabled, err := s.matchRepository(bufferedCtx, workDir, metadata.GetEnv(), metadata.GetAppRelPath())
	if err != nil {
		return fmt.Errorf("match repository error: %w", err)
	}
	repoResponse := &apiclient.RepositoryResponse{IsSupported: isSupported, IsDiscoveryEnabled: isDiscoveryEnabled}

	err = stream.SendAndClose(repoResponse)
	if err != nil {
		return fmt.Errorf("error sending match repository response: %w", err)
	}
	return nil
}

func (s *Service) matchRepository(ctx context.Context, workdir string, envEntries []*apiclient.EnvEntry, appRelPath string) (isSupported bool, isDiscoveryEnabled bool, err error) {
	config := s.initConstants.PluginConfig

	appPath, err := securejoin.SecureJoin(workdir, appRelPath)
	if err != nil {
		log.WithFields(map[string]any{
			common.SecurityField:    common.SecurityHigh,
			common.SecurityCWEField: common.SecurityCWEIncompleteCleanup,
		}).Errorf("error joining workdir %q and appRelPath %q: %v", workdir, appRelPath, err)
	}

	if config.Spec.Discover.FileName != "" {
		log.Debugf("config.Spec.Discover.FileName is provided")
		pattern := filepath.Join(appPath, config.Spec.Discover.FileName)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			e := fmt.Errorf("error finding filename match for pattern %q: %w", pattern, err)
			log.Debug(e)
			return false, true, e
		}
		return len(matches) > 0, true, nil
	}

	if config.Spec.Discover.Find.Glob != "" {
		log.Debugf("config.Spec.Discover.Find.Glob is provided")
		pattern := filepath.Join(appPath, config.Spec.Discover.Find.Glob)
		// filepath.Glob doesn't have '**' support hence selecting third-party lib
		// https://github.com/golang/go/issues/11862
		matches, err := zglob.Glob(pattern)
		if err != nil {
			e := fmt.Errorf("error finding glob match for pattern %q: %w", pattern, err)
			log.Debug(e)
			return false, true, e
		}

		return len(matches) > 0, true, nil
	}

	if len(config.Spec.Discover.Find.Command.Command) > 0 {
		log.Debugf("Going to try runCommand.")
		env := append(os.Environ(), environ(envEntries)...)
		find, err := runCommand(ctx, config.Spec.Discover.Find.Command, appPath, env)
		if err != nil {
			return false, true, fmt.Errorf("error running find command: %w", err)
		}
		return find != "", true, nil
	}

	return false, false, nil
}

// ParametersAnnouncementStream defines an interface able to send/receive a stream of parameter announcements.
type ParametersAnnouncementStream interface {
	Stream
	SendAndClose(response *apiclient.ParametersAnnouncementResponse) error
}

// GetParametersAnnouncement gets parameter announcements for a given Application and repo contents.
func (s *Service) GetParametersAnnouncement(stream apiclient.ConfigManagementPluginService_GetParametersAnnouncementServer) error {
	bufferedCtx, cancel := buffered_context.WithEarlierDeadline(stream.Context(), cmpTimeoutBuffer)
	defer cancel()

	workDir, cleanup, err := getTempDirMustCleanup(common.GetCMPWorkDir())
	if err != nil {
		return fmt.Errorf("error creating workdir for generating parameter announcements: %w", err)
	}
	defer cleanup()

	metadata, err := cmp.ReceiveRepoStream(bufferedCtx, stream, workDir, s.initConstants.PluginConfig.Spec.PreserveFileMode)
	if err != nil {
		return fmt.Errorf("parameters announcement error receiving stream: %w", err)
	}
	appPath := filepath.Clean(filepath.Join(workDir, metadata.AppRelPath))
	if !strings.HasPrefix(appPath, workDir) {
		return errors.New("illegal appPath: out of workDir bound")
	}

	repoResponse, err := getParametersAnnouncement(bufferedCtx, appPath, s.initConstants.PluginConfig.Spec.Parameters.Static, s.initConstants.PluginConfig.Spec.Parameters.Dynamic, metadata.GetEnv())
	if err != nil {
		return fmt.Errorf("get parameters announcement error: %w", err)
	}

	err = stream.SendAndClose(repoResponse)
	if err != nil {
		return fmt.Errorf("error sending parameters announcement response: %w", err)
	}
	return nil
}

func getParametersAnnouncement(ctx context.Context, appDir string, announcements []*repoclient.ParameterAnnouncement, command Command, envEntries []*apiclient.EnvEntry) (*apiclient.ParametersAnnouncementResponse, error) {
	augmentedAnnouncements := announcements

	if len(command.Command) > 0 {
		env := append(os.Environ(), environ(envEntries)...)
		stdout, err := runCommand(ctx, command, appDir, env)
		if err != nil {
			return nil, fmt.Errorf("error executing dynamic parameter output command: %w", err)
		}

		var dynamicParamAnnouncements []*repoclient.ParameterAnnouncement
		err = json.Unmarshal([]byte(stdout), &dynamicParamAnnouncements)
		if err != nil {
			return nil, fmt.Errorf("error unmarshaling dynamic parameter output into ParametersAnnouncementResponse: %w", err)
		}

		// dynamic goes first, because static should take precedence by being later.
		augmentedAnnouncements = append(dynamicParamAnnouncements, announcements...)
	}

	repoResponse := &apiclient.ParametersAnnouncementResponse{
		ParameterAnnouncements: augmentedAnnouncements,
	}
	return repoResponse, nil
}

func (s *Service) CheckPluginConfiguration(_ context.Context, _ *empty.Empty) (*apiclient.CheckPluginConfigurationResponse, error) {
	isDiscoveryConfigured := s.isDiscoveryConfigured()
	response := &apiclient.CheckPluginConfigurationResponse{IsDiscoveryConfigured: isDiscoveryConfigured, ProvideGitCreds: s.initConstants.PluginConfig.Spec.ProvideGitCreds}

	return response, nil
}

func (s *Service) isDiscoveryConfigured() (isDiscoveryConfigured bool) {
	config := s.initConstants.PluginConfig
	return config.Spec.Discover.FileName != "" || config.Spec.Discover.Find.Glob != "" || len(config.Spec.Discover.Find.Command.Command) > 0
}
