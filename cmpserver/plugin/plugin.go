package plugin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/argoproj/pkg/rand"

	"github.com/argoproj/argo-cd/v2/util/buffered_context"
	"github.com/argoproj/argo-cd/v2/util/files"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/mattn/go-zglob"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v2/cmpserver/apiclient"
)

// cmpTimeoutBuffer is the amount of time before the request deadline to timeout server-side work. It makes sure there's
// enough time before the client times out to send a meaningful error message.
const cmpTimeoutBuffer = 500 * time.Millisecond

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
func (s *Service) GenerateManifest(stream apiclient.ConfigManagementPluginService_GenerateManifestServer) error {
	workDir, env, err := ReceiveStream(stream.Context(), stream)
	if err != nil {
		return fmt.Errorf("generate manifest error receiving stream: %s", err)
	}
	defer os.RemoveAll(workDir)

	response, err := s.generateManifest(stream.Context(), workDir, env)
	if err != nil {
		return fmt.Errorf("error generating manifests: %s", err)
	}
	err = stream.SendAndClose(response)
	if err != nil {
		return fmt.Errorf("error sending manifest response: %s", err)
	}
	return nil
}

// Receiver defines the contract for receiving Application's files
// over gRPC stream
type Receiver interface {
	Recv() (*apiclient.AppStreamRequest, error)
}

// ReceiveStream will receive the Application's files and the env entries
// over the gRPC stream. Will return the path where the files are saved
// and the env entries if no error.
func ReceiveStream(ctx context.Context, receiver Receiver) (string, []*apiclient.EnvEntry, error) {
	header, err := receiver.Recv()
	if err != nil {
		return "", nil, fmt.Errorf("error receiving stream header: %s", err)
	}
	if header == nil || header.GetMetadata() == nil {
		return "", nil, fmt.Errorf("error getting stream metadata: metadata is nil")
	}
	metadata := header.GetMetadata()
	workDir, err := ioutil.TempDir(os.TempDir(), metadata.GetAppName())
	if err != nil {
		return "", nil, fmt.Errorf("error creating workDir: %s", err)
	}

	tgzFile, err := receiveFile(ctx, receiver, metadata.GetChecksum(), workDir)
	if err != nil {
		return "", nil, fmt.Errorf("error receiving file: %s", err)
	}
	err = files.Untgz(workDir, tgzFile)
	if err != nil {
		return "", nil, fmt.Errorf("error decompressing tgz file: %s", err)
	}
	err = os.Remove(tgzFile.Name())
	if err != nil {
		log.Warnf("error removing the tgz file %q: %s", tgzFile.Name, err)
	}
	return workDir, metadata.GetEnv(), nil
}

// receiveFile will receive the file from the gRPC stream and save it in the dst folder.
// Returns error if checksum doesn't match the one provided in the fileMetadata.
// It is responsibility of the caller to close the returned file.
func receiveFile(ctx context.Context, receiver Receiver, checksum, dst string) (*os.File, error) {
	fileBuffer := bytes.Buffer{}
	hasher := sha256.New()
	for {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("stream context error: %s", err)
			}
		}
		req, err := receiver.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("stream Recv error: %s", err)
		}
		f := req.GetFile()
		if f == nil {
			return nil, fmt.Errorf("stream request file is nil")
		}
		_, err = fileBuffer.Write(f.Chunk)
		if err != nil {
			return nil, fmt.Errorf("error writing file buffer: %s", err)
		}
		_, err = hasher.Write(f.Chunk)
		if err != nil {
			return nil, fmt.Errorf("error writing hasher: %s", err)
		}
	}
	if hex.EncodeToString(hasher.Sum(nil)) != checksum {
		return nil, fmt.Errorf("file checksum validation error")
	}

	tgzFile, err := ioutil.TempFile(dst, "")
	if err != nil {
		return nil, fmt.Errorf("error creating tgz file: %s", err)
	}
	_, err = fileBuffer.WriteTo(tgzFile)
	if err != nil {
		return nil, fmt.Errorf("error writing tgz file: %s", err)
	}
	_, err = tgzFile.Seek(0, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("tgz seek error: %s", err)
	}
	return tgzFile, nil
}

// generateManifest runs generate command from plugin config file and returns generated manifest files
func (s *Service) generateManifest(ctx context.Context, workDir string, envEntries []*apiclient.EnvEntry) (*apiclient.ManifestResponse, error) {
	bufferedCtx, cancel := buffered_context.WithEarlierDeadline(ctx, cmpTimeoutBuffer)
	defer cancel()

	if deadline, ok := bufferedCtx.Deadline(); ok {
		log.Infof("Generating manifests with deadline %v from now", time.Until(deadline))
	} else {
		log.Info("Generating manifests with no request-level timeout")
	}

	config := s.initConstants.PluginConfig

	env := append(os.Environ(), environ(envEntries)...)
	if len(config.Spec.Init.Command) > 0 {
		_, err := runCommand(bufferedCtx, config.Spec.Init, workDir, env)
		if err != nil {
			return &apiclient.ManifestResponse{}, err
		}
	}

	out, err := runCommand(bufferedCtx, config.Spec.Generate, workDir, env)
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

// MatchRepository checks whether the application repository type is supported
// by config management plugin server. The checks are implemented in the following
// order:
// 1. If spec.Discover.FileName is provided it finds for a name match in Applications files
// 2. If spec.Discover.Find.Glob is provided if finds for a glob match in Applications files
// 3. Otherwise it runs the spec.Discover.Find.Command
func (s *Service) MatchRepository(stream apiclient.ConfigManagementPluginService_MatchRepositoryServer) error {
	bufferedCtx, cancel := buffered_context.WithEarlierDeadline(stream.Context(), cmpTimeoutBuffer)
	defer cancel()
	workdir, _, err := ReceiveStream(bufferedCtx, stream)
	if err != nil {
		return fmt.Errorf("match repository error receiving stream: %s", err)
	}
	defer os.RemoveAll(workdir)

	repoResponse := &apiclient.RepositoryResponse{}
	config := s.initConstants.PluginConfig
	if config.Spec.Discover.FileName != "" {
		log.Debugf("config.Spec.Discover.FileName is provided")
		pattern := filepath.Join(workdir, config.Spec.Discover.FileName)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			e := fmt.Errorf("error finding filename match for pattern %q: %s", pattern, err)
			log.Debug(e)
			return e
		}
		if len(matches) > 0 {
			repoResponse.IsSupported = true
		}
		err = stream.SendAndClose(repoResponse)
		if err != nil {
			return fmt.Errorf("error closing stream: %s", err)
		}
		return nil
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
			return e
		}

		if len(matches) > 0 {
			repoResponse.IsSupported = true
		}
		err = stream.SendAndClose(repoResponse)
		if err != nil {
			return fmt.Errorf("error closing stream: %s", err)
		}
		return nil
	}

	log.Debugf("Going to try runCommand.")
	find, err := runCommand(bufferedCtx, config.Spec.Discover.Find.Command, workdir, os.Environ())
	if err != nil {
		return fmt.Errorf("error running find command: %s", err)
	}

	if find != "" {
		repoResponse.IsSupported = true
	}
	err = stream.SendAndClose(repoResponse)
	if err != nil {
		return fmt.Errorf("error closing stream: %s", err)
	}
	return nil
}

// GetPluginConfig returns plugin config
func (s *Service) GetPluginConfig(ctx context.Context, q *apiclient.ConfigRequest) (*apiclient.ConfigResponse, error) {
	config := s.initConstants.PluginConfig
	return &apiclient.ConfigResponse{
		AllowConcurrency: config.Spec.AllowConcurrency,
		LockRepo:         config.Spec.LockRepo,
	}, nil
}
