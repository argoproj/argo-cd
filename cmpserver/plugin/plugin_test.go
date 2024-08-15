package plugin

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/cmpserver/apiclient"
	repoclient "github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/test"
	"github.com/argoproj/argo-cd/v2/util/cmp"
	"github.com/argoproj/argo-cd/v2/util/tgzstream"
)

func newService(configFilePath string) (*Service, error) {
	config, err := ReadPluginConfig(configFilePath)
	if err != nil {
		return nil, err
	}

	initConstants := CMPServerInitConstants{
		PluginConfig: *config,
	}

	service := &Service{
		initConstants: initConstants,
	}
	return service, nil
}

func (s *Service) WithGenerateCommand(command Command) *Service {
	s.initConstants.PluginConfig.Spec.Generate = command
	return s
}

type pluginOpt func(*CMPServerInitConstants)

func withDiscover(d Discover) pluginOpt {
	return func(cic *CMPServerInitConstants) {
		cic.PluginConfig.Spec.Discover = d
	}
}

func buildPluginConfig(opts ...pluginOpt) *CMPServerInitConstants {
	cic := &CMPServerInitConstants{
		PluginConfig: PluginConfig{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigManagementPlugin",
				APIVersion: "argoproj.io/v1alpha1",
			},
			Metadata: metav1.ObjectMeta{
				Name: "some-plugin",
			},
			Spec: PluginConfigSpec{
				Version: "v1.0",
			},
		},
	}
	for _, opt := range opts {
		opt(cic)
	}
	return cic
}

func TestMatchRepository(t *testing.T) {
	type fixture struct {
		service *Service
		path    string
		env     []*apiclient.EnvEntry
	}
	setup := func(t *testing.T, opts ...pluginOpt) *fixture {
		t.Helper()
		cic := buildPluginConfig(opts...)
		path := filepath.Join(test.GetTestDir(t), "testdata", "kustomize")
		s := NewService(*cic)
		return &fixture{
			service: s,
			path:    path,
			env:     []*apiclient.EnvEntry{{Name: "ENV_VAR", Value: "1"}},
		}
	}
	t.Run("will match plugin by filename", func(t *testing.T) {
		// given
		d := Discover{
			FileName: "kustomization.yaml",
		}
		f := setup(t, withDiscover(d))

		// when
		match, discovery, err := f.service.matchRepository(context.Background(), f.path, f.env, ".")

		// then
		require.NoError(t, err)
		assert.True(t, match)
		assert.True(t, discovery)
	})
	t.Run("will not match plugin by filename if file not found", func(t *testing.T) {
		// given
		d := Discover{
			FileName: "not_found.yaml",
		}
		f := setup(t, withDiscover(d))

		// when
		match, discovery, err := f.service.matchRepository(context.Background(), f.path, f.env, ".")

		// then
		require.NoError(t, err)
		assert.False(t, match)
		assert.True(t, discovery)
	})
	t.Run("will not match a pattern with a syntax error", func(t *testing.T) {
		// given
		d := Discover{
			FileName: "[",
		}
		f := setup(t, withDiscover(d))

		// when
		_, _, err := f.service.matchRepository(context.Background(), f.path, f.env, ".")

		// then
		require.ErrorContains(t, err, "syntax error")
	})
	t.Run("will match plugin by glob", func(t *testing.T) {
		// given
		d := Discover{
			Find: Find{
				Glob: "**/*/plugin.yaml",
			},
		}
		f := setup(t, withDiscover(d))

		// when
		match, discovery, err := f.service.matchRepository(context.Background(), f.path, f.env, ".")

		// then
		require.NoError(t, err)
		assert.True(t, match)
		assert.True(t, discovery)
	})
	t.Run("will not match plugin by glob if not found", func(t *testing.T) {
		// given
		d := Discover{
			Find: Find{
				Glob: "**/*/not_found.yaml",
			},
		}
		f := setup(t, withDiscover(d))

		// when
		match, discovery, err := f.service.matchRepository(context.Background(), f.path, f.env, ".")

		// then
		require.NoError(t, err)
		assert.False(t, match)
		assert.True(t, discovery)
	})
	t.Run("will throw an error for a bad pattern", func(t *testing.T) {
		// given
		d := Discover{
			Find: Find{
				Glob: "does-not-exist",
			},
		}
		f := setup(t, withDiscover(d))

		// when
		_, _, err := f.service.matchRepository(context.Background(), f.path, f.env, ".")

		// then
		require.ErrorContains(t, err, "error finding glob match for pattern")
	})
	t.Run("will match plugin by command when returns any output", func(t *testing.T) {
		// given
		d := Discover{
			Find: Find{
				Command: Command{
					Command: []string{"echo", "test"},
				},
			},
		}
		f := setup(t, withDiscover(d))

		// when
		match, discovery, err := f.service.matchRepository(context.Background(), f.path, f.env, ".")

		// then
		require.NoError(t, err)
		assert.True(t, match)
		assert.True(t, discovery)
	})
	t.Run("will not match plugin by command when returns no output", func(t *testing.T) {
		// given
		d := Discover{
			Find: Find{
				Command: Command{
					Command: []string{"echo"},
				},
			},
		}
		f := setup(t, withDiscover(d))

		// when
		match, discovery, err := f.service.matchRepository(context.Background(), f.path, f.env, ".")
		// then
		require.NoError(t, err)
		assert.False(t, match)
		assert.True(t, discovery)
	})
	t.Run("will match plugin because env var defined", func(t *testing.T) {
		// given
		d := Discover{
			Find: Find{
				Command: Command{
					Command: []string{"sh", "-c", "echo -n $ENV_VAR"},
				},
			},
		}
		f := setup(t, withDiscover(d))

		// when
		match, discovery, err := f.service.matchRepository(context.Background(), f.path, f.env, ".")

		// then
		require.NoError(t, err)
		assert.True(t, match)
		assert.True(t, discovery)
	})
	t.Run("will not match plugin because no env var defined", func(t *testing.T) {
		// given
		d := Discover{
			Find: Find{
				Command: Command{
					// Use printf instead of echo since OSX prints the "-n" when there's no additional arg.
					Command: []string{"sh", "-c", `printf "%s" "$ENV_NO_VAR"`},
				},
			},
		}
		f := setup(t, withDiscover(d))

		// when
		match, discovery, err := f.service.matchRepository(context.Background(), f.path, f.env, ".")

		// then
		require.NoError(t, err)
		assert.False(t, match)
		assert.True(t, discovery)
	})
	t.Run("will not match plugin by command when command fails", func(t *testing.T) {
		// given
		d := Discover{
			Find: Find{
				Command: Command{
					Command: []string{"cat", "nil"},
				},
			},
		}
		f := setup(t, withDiscover(d))

		// when
		match, discovery, err := f.service.matchRepository(context.Background(), f.path, f.env, ".")

		// then
		require.Error(t, err)
		assert.False(t, match)
		assert.True(t, discovery)
	})
	t.Run("will not match plugin as discovery is not set", func(t *testing.T) {
		// given
		d := Discover{}
		f := setup(t, withDiscover(d))

		// when
		match, discovery, err := f.service.matchRepository(context.Background(), f.path, f.env, ".")

		// then
		require.NoError(t, err)
		assert.False(t, match)
		assert.False(t, discovery)
	})
}

func Test_Negative_ConfigFile_DoesnotExist(t *testing.T) {
	configFilePath := "./testdata/kustomize-neg/config"
	service, err := newService(configFilePath)
	require.Error(t, err)
	require.Nil(t, service)
}

func TestGenerateManifest(t *testing.T) {
	configFilePath := "./testdata/kustomize/config"

	t.Run("successful generate", func(t *testing.T) {
		service, err := newService(configFilePath)
		require.NoError(t, err)

		res1, err := service.generateManifest(context.Background(), "testdata/kustomize", nil)
		require.NoError(t, err)
		require.NotNil(t, res1)

		expectedOutput := "{\"apiVersion\":\"v1\",\"data\":{\"foo\":\"bar\"},\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"my-map\"}}"
		if res1 != nil {
			require.Equal(t, expectedOutput, res1.Manifests[0])
		}
	})
	t.Run("bad generate command", func(t *testing.T) {
		service, err := newService(configFilePath)
		require.NoError(t, err)
		service.WithGenerateCommand(Command{Command: []string{"bad-command"}})

		res, err := service.generateManifest(context.Background(), "testdata/kustomize", nil)
		require.ErrorContains(t, err, "executable file not found")
		assert.Nil(t, res.Manifests)
	})
	t.Run("bad yaml output", func(t *testing.T) {
		service, err := newService(configFilePath)
		require.NoError(t, err)
		service.WithGenerateCommand(Command{Command: []string{"echo", "invalid yaml: }"}})

		res, err := service.generateManifest(context.Background(), "testdata/kustomize", nil)
		require.ErrorContains(t, err, "failed to unmarshal manifest")
		assert.Nil(t, res.Manifests)
	})
}

func TestGenerateManifest_deadline_exceeded(t *testing.T) {
	configFilePath := "./testdata/kustomize/config"
	service, err := newService(configFilePath)
	require.NoError(t, err)

	expiredCtx, cancel := context.WithTimeout(context.Background(), time.Second*0)
	defer cancel()
	_, err = service.generateManifest(expiredCtx, "", nil)
	require.ErrorContains(t, err, "context deadline exceeded")
}

// TestRunCommandContextTimeout makes sure the command dies at timeout rather than sleeping past the timeout.
func TestRunCommandContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 990*time.Millisecond)
	defer cancel()
	// Use a subshell so there's a child command.
	command := Command{
		Command: []string{"sh", "-c"},
		Args:    []string{"sleep 5"},
	}
	before := time.Now()
	_, err := runCommand(ctx, command, "", []string{})
	after := time.Now()
	require.Error(t, err) // The command should time out, causing an error.
	assert.Less(t, after.Sub(before), 1*time.Second)
}

func TestRunCommandEmptyCommand(t *testing.T) {
	_, err := runCommand(context.Background(), Command{}, "", nil)
	require.ErrorContains(t, err, "Command is empty")
}

// TestRunCommandContextTimeoutWithCleanup makes sure that the process is given enough time to cleanup before sending SIGKILL.
func TestRunCommandContextTimeoutWithCleanup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 900*time.Millisecond)
	defer cancel()

	// Use a subshell so there's a child command.
	// This command sleeps for 4 seconds which is currently less than the 5 second delay between SIGTERM and SIGKILL signal and then exits successfully.
	command := Command{
		Command: []string{"sh", "-c"},
		Args:    []string{`(trap 'echo "cleanup completed"; exit' TERM; sleep 4)`},
	}

	before := time.Now()
	output, err := runCommand(ctx, command, "", []string{})
	after := time.Now()

	require.Error(t, err) // The command should time out, causing an error.
	assert.Less(t, after.Sub(before), 1*time.Second)
	// The command should still have completed the cleanup after termination.
	assert.Contains(t, output, "cleanup completed")
}

func Test_getParametersAnnouncement_empty_command(t *testing.T) {
	staticYAML := `
- name: static-a
- name: static-b
`
	static := &[]*repoclient.ParameterAnnouncement{}
	err := yaml.Unmarshal([]byte(staticYAML), static)
	require.NoError(t, err)
	command := Command{
		Command: []string{"echo"},
		Args:    []string{`[]`},
	}
	res, err := getParametersAnnouncement(context.Background(), "", *static, command, []*apiclient.EnvEntry{})
	require.NoError(t, err)
	assert.Equal(t, []*repoclient.ParameterAnnouncement{{Name: "static-a"}, {Name: "static-b"}}, res.ParameterAnnouncements)
}

func Test_getParametersAnnouncement_no_command(t *testing.T) {
	staticYAML := `
- name: static-a
- name: static-b
`
	static := &[]*repoclient.ParameterAnnouncement{}
	err := yaml.Unmarshal([]byte(staticYAML), static)
	require.NoError(t, err)
	command := Command{}
	res, err := getParametersAnnouncement(context.Background(), "", *static, command, []*apiclient.EnvEntry{})
	require.NoError(t, err)
	assert.Equal(t, []*repoclient.ParameterAnnouncement{{Name: "static-a"}, {Name: "static-b"}}, res.ParameterAnnouncements)
}

func Test_getParametersAnnouncement_static_and_dynamic(t *testing.T) {
	staticYAML := `
- name: static-a
- name: static-b
`
	static := &[]*repoclient.ParameterAnnouncement{}
	err := yaml.Unmarshal([]byte(staticYAML), static)
	require.NoError(t, err)
	command := Command{
		Command: []string{"echo"},
		Args:    []string{`[{"name": "dynamic-a"}, {"name": "dynamic-b"}]`},
	}
	res, err := getParametersAnnouncement(context.Background(), "", *static, command, []*apiclient.EnvEntry{})
	require.NoError(t, err)
	expected := []*repoclient.ParameterAnnouncement{
		{Name: "dynamic-a"},
		{Name: "dynamic-b"},
		{Name: "static-a"},
		{Name: "static-b"},
	}
	assert.Equal(t, expected, res.ParameterAnnouncements)
}

func Test_getParametersAnnouncement_invalid_json(t *testing.T) {
	command := Command{
		Command: []string{"echo"},
		Args:    []string{`[`},
	}
	_, err := getParametersAnnouncement(context.Background(), "", []*repoclient.ParameterAnnouncement{}, command, []*apiclient.EnvEntry{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected end of JSON input")
}

func Test_getParametersAnnouncement_bad_command(t *testing.T) {
	command := Command{
		Command: []string{"exit"},
		Args:    []string{"1"},
	}
	_, err := getParametersAnnouncement(context.Background(), "", []*repoclient.ParameterAnnouncement{}, command, []*apiclient.EnvEntry{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error executing dynamic parameter output command")
}

func Test_getTempDirMustCleanup(t *testing.T) {
	tempDir := t.TempDir()

	// Induce a directory create error to verify error handling.
	err := os.Chmod(tempDir, 0o000)
	require.NoError(t, err)
	_, _, err = getTempDirMustCleanup(path.Join(tempDir, "test"))
	require.ErrorContains(t, err, "error creating temp dir")

	err = os.Chmod(tempDir, 0o700)
	require.NoError(t, err)
	workDir, cleanup, err := getTempDirMustCleanup(tempDir)
	require.NoError(t, err)
	require.DirExists(t, workDir)
	cleanup()
	assert.NoDirExists(t, workDir)
}

func TestService_Init(t *testing.T) {
	// Set up a base directory containing a test directory and a test file.
	tempDir := t.TempDir()
	workDir := path.Join(tempDir, "workDir")
	err := os.MkdirAll(workDir, 0o700)
	require.NoError(t, err)
	testfile := path.Join(workDir, "testfile")
	file, err := os.Create(testfile)
	require.NoError(t, err)
	err = file.Close()
	require.NoError(t, err)

	// Make the base directory read-only so Init's cleanup fails.
	err = os.Chmod(tempDir, 0o000)
	require.NoError(t, err)
	s := NewService(CMPServerInitConstants{PluginConfig: PluginConfig{}})
	err = s.Init(workDir)
	require.ErrorContains(t, err, "error removing workdir", "Init must throw an error if it can't remove the work directory")

	// Make the base directory writable so Init's cleanup succeeds.
	err = os.Chmod(tempDir, 0o700)
	require.NoError(t, err)
	err = s.Init(workDir)
	require.NoError(t, err)
	assert.DirExists(t, workDir)
	assert.NoFileExists(t, testfile)
}

func TestEnviron(t *testing.T) {
	t.Run("empty environ", func(t *testing.T) {
		env := environ([]*apiclient.EnvEntry{})
		assert.Nil(t, env)
	})
	t.Run("env vars with empty names or values", func(t *testing.T) {
		env := environ([]*apiclient.EnvEntry{
			{Value: "test"},
			{Name: "test"},
		})
		assert.Nil(t, env)
	})
	t.Run("proper env vars", func(t *testing.T) {
		env := environ([]*apiclient.EnvEntry{
			{Name: "name1", Value: "value1"},
			{Name: "name2", Value: "value2"},
		})
		assert.Equal(t, []string{"name1=value1", "name2=value2"}, env)
	})
}

func TestIsDiscoveryConfigured(t *testing.T) {
	type fixture struct {
		service *Service
	}
	setup := func(t *testing.T, opts ...pluginOpt) *fixture {
		t.Helper()
		cic := buildPluginConfig(opts...)
		s := NewService(*cic)
		return &fixture{
			service: s,
		}
	}
	t.Run("discovery is enabled when is configured by FileName", func(t *testing.T) {
		// given
		d := Discover{
			FileName: "kustomization.yaml",
		}
		f := setup(t, withDiscover(d))

		// when
		isDiscoveryConfigured := f.service.isDiscoveryConfigured()

		// then
		assert.True(t, isDiscoveryConfigured)
	})
	t.Run("discovery is enabled when is configured by Glob", func(t *testing.T) {
		// given
		d := Discover{
			Find: Find{
				Glob: "**/*/plugin.yaml",
			},
		}
		f := setup(t, withDiscover(d))

		// when
		isDiscoveryConfigured := f.service.isDiscoveryConfigured()

		// then
		assert.True(t, isDiscoveryConfigured)
	})
	t.Run("discovery is enabled when is configured by Command", func(t *testing.T) {
		// given
		d := Discover{
			Find: Find{
				Command: Command{
					Command: []string{"echo", "test"},
				},
			},
		}
		f := setup(t, withDiscover(d))

		// when
		isDiscoveryConfigured := f.service.isDiscoveryConfigured()

		// then
		assert.True(t, isDiscoveryConfigured)
	})
	t.Run("discovery is disabled when discover is not configured", func(t *testing.T) {
		// given
		d := Discover{}
		f := setup(t, withDiscover(d))

		// when
		isDiscoveryConfigured := f.service.isDiscoveryConfigured()

		// then
		assert.False(t, isDiscoveryConfigured)
	})
}

type MockGenerateManifestStream struct {
	metadataSent    bool
	fileSent        bool
	metadataRequest *apiclient.AppStreamRequest
	fileRequest     *apiclient.AppStreamRequest
	response        *apiclient.ManifestResponse
}

func NewMockGenerateManifestStream(repoPath, appPath string, env []string) (*MockGenerateManifestStream, error) {
	tgz, mr, err := cmp.GetCompressedRepoAndMetadata(repoPath, appPath, env, nil, nil)
	if err != nil {
		return nil, err
	}
	defer tgzstream.CloseAndDelete(tgz)

	tgzBuffer := bytes.NewBuffer(nil)
	_, err = io.Copy(tgzBuffer, tgz)
	if err != nil {
		return nil, fmt.Errorf("failed to copy manifest targz to a byte buffer: %w", err)
	}

	return &MockGenerateManifestStream{
		metadataRequest: mr,
		fileRequest:     cmp.AppFileRequest(tgzBuffer.Bytes()),
	}, nil
}

func (m *MockGenerateManifestStream) SendAndClose(response *apiclient.ManifestResponse) error {
	m.response = response
	return nil
}

func (m *MockGenerateManifestStream) Recv() (*apiclient.AppStreamRequest, error) {
	if !m.metadataSent {
		m.metadataSent = true
		return m.metadataRequest, nil
	}

	if !m.fileSent {
		m.fileSent = true
		return m.fileRequest, nil
	}
	return nil, io.EOF
}

func (m *MockGenerateManifestStream) Context() context.Context {
	return context.Background()
}

func TestService_GenerateManifest(t *testing.T) {
	configFilePath := "./testdata/kustomize/config"
	service, err := newService(configFilePath)
	require.NoError(t, err)

	t.Run("successful generate", func(t *testing.T) {
		s, err := NewMockGenerateManifestStream("./testdata/kustomize", "./testdata/kustomize", nil)
		require.NoError(t, err)
		err = service.generateManifestGeneric(s)
		require.NoError(t, err)
		require.NotNil(t, s.response)
		assert.Equal(t, []string{"{\"apiVersion\":\"v1\",\"data\":{\"foo\":\"bar\"},\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"my-map\"}}"}, s.response.Manifests)
	})

	t.Run("out-of-bounds app path", func(t *testing.T) {
		s, err := NewMockGenerateManifestStream("./testdata/kustomize", "./testdata/kustomize", nil)
		require.NoError(t, err)
		// set a malicious app path on the metadata
		s.metadataRequest.Request.(*apiclient.AppStreamRequest_Metadata).Metadata.AppRelPath = "../out-of-bounds"
		err = service.generateManifestGeneric(s)
		require.ErrorContains(t, err, "illegal appPath")
		assert.Nil(t, s.response)
	})
}

type MockMatchRepositoryStream struct {
	metadataSent    bool
	fileSent        bool
	metadataRequest *apiclient.AppStreamRequest
	fileRequest     *apiclient.AppStreamRequest
	response        *apiclient.RepositoryResponse
}

func NewMockMatchRepositoryStream(repoPath, appPath string, env []string) (*MockMatchRepositoryStream, error) {
	tgz, mr, err := cmp.GetCompressedRepoAndMetadata(repoPath, appPath, env, nil, nil)
	if err != nil {
		return nil, err
	}
	defer tgzstream.CloseAndDelete(tgz)

	tgzBuffer := bytes.NewBuffer(nil)
	_, err = io.Copy(tgzBuffer, tgz)
	if err != nil {
		return nil, fmt.Errorf("failed to copy manifest targz to a byte buffer: %w", err)
	}

	return &MockMatchRepositoryStream{
		metadataRequest: mr,
		fileRequest:     cmp.AppFileRequest(tgzBuffer.Bytes()),
	}, nil
}

func (m *MockMatchRepositoryStream) SendAndClose(response *apiclient.RepositoryResponse) error {
	m.response = response
	return nil
}

func (m *MockMatchRepositoryStream) Recv() (*apiclient.AppStreamRequest, error) {
	if !m.metadataSent {
		m.metadataSent = true
		return m.metadataRequest, nil
	}

	if !m.fileSent {
		m.fileSent = true
		return m.fileRequest, nil
	}
	return nil, io.EOF
}

func (m *MockMatchRepositoryStream) Context() context.Context {
	return context.Background()
}

func TestService_MatchRepository(t *testing.T) {
	configFilePath := "./testdata/kustomize/config"
	service, err := newService(configFilePath)
	require.NoError(t, err)

	t.Run("supported app", func(t *testing.T) {
		s, err := NewMockMatchRepositoryStream("./testdata/kustomize", "./testdata/kustomize", nil)
		require.NoError(t, err)
		err = service.matchRepositoryGeneric(s)
		require.NoError(t, err)
		require.NotNil(t, s.response)
		assert.True(t, s.response.IsSupported)
	})

	t.Run("unsupported app", func(t *testing.T) {
		s, err := NewMockMatchRepositoryStream("./testdata/ksonnet", "./testdata/ksonnet", nil)
		require.NoError(t, err)
		err = service.matchRepositoryGeneric(s)
		require.NoError(t, err)
		require.NotNil(t, s.response)
		assert.False(t, s.response.IsSupported)
	})
}

type MockParametersAnnouncementStream struct {
	metadataSent    bool
	fileSent        bool
	metadataRequest *apiclient.AppStreamRequest
	fileRequest     *apiclient.AppStreamRequest
	response        *apiclient.ParametersAnnouncementResponse
}

func NewMockParametersAnnouncementStream(repoPath, appPath string, env []string) (*MockParametersAnnouncementStream, error) {
	tgz, mr, err := cmp.GetCompressedRepoAndMetadata(repoPath, appPath, env, nil, nil)
	if err != nil {
		return nil, err
	}
	defer tgzstream.CloseAndDelete(tgz)

	tgzBuffer := bytes.NewBuffer(nil)
	_, err = io.Copy(tgzBuffer, tgz)
	if err != nil {
		return nil, fmt.Errorf("failed to copy manifest targz to a byte buffer: %w", err)
	}

	return &MockParametersAnnouncementStream{
		metadataRequest: mr,
		fileRequest:     cmp.AppFileRequest(tgzBuffer.Bytes()),
	}, nil
}

func (m *MockParametersAnnouncementStream) SendAndClose(response *apiclient.ParametersAnnouncementResponse) error {
	m.response = response
	return nil
}

func (m *MockParametersAnnouncementStream) Recv() (*apiclient.AppStreamRequest, error) {
	if !m.metadataSent {
		m.metadataSent = true
		return m.metadataRequest, nil
	}

	if !m.fileSent {
		m.fileSent = true
		return m.fileRequest, nil
	}
	return nil, io.EOF
}

func (m *MockParametersAnnouncementStream) SetHeader(metadata.MD) error {
	return nil
}

func (m *MockParametersAnnouncementStream) SendHeader(metadata.MD) error {
	return nil
}

func (m *MockParametersAnnouncementStream) SetTrailer(metadata.MD) {}

func (m *MockParametersAnnouncementStream) Context() context.Context {
	return context.Background()
}

func (m *MockParametersAnnouncementStream) SendMsg(interface{}) error {
	return nil
}

func (m *MockParametersAnnouncementStream) RecvMsg(interface{}) error {
	return nil
}

func TestService_GetParametersAnnouncement(t *testing.T) {
	configFilePath := "./testdata/kustomize/config"
	service, err := newService(configFilePath)
	require.NoError(t, err)

	t.Run("successful response", func(t *testing.T) {
		s, err := NewMockParametersAnnouncementStream("./testdata/kustomize", "./testdata/kustomize", []string{"MUST_BE_SET=yep"})
		require.NoError(t, err)
		err = service.GetParametersAnnouncement(s)
		require.NoError(t, err)
		require.NotNil(t, s.response)
		require.Len(t, s.response.ParameterAnnouncements, 2)
		assert.Equal(t, repoclient.ParameterAnnouncement{Name: "dynamic-test-param", String_: "yep"}, *s.response.ParameterAnnouncements[0])
		assert.Equal(t, repoclient.ParameterAnnouncement{Name: "test-param", String_: "test-value"}, *s.response.ParameterAnnouncements[1])
	})
	t.Run("out of bounds app", func(t *testing.T) {
		s, err := NewMockParametersAnnouncementStream("./testdata/kustomize", "./testdata/kustomize", []string{"MUST_BE_SET=yep"})
		require.NoError(t, err)
		// set a malicious app path on the metadata
		s.metadataRequest.Request.(*apiclient.AppStreamRequest_Metadata).Metadata.AppRelPath = "../out-of-bounds"
		err = service.GetParametersAnnouncement(s)
		require.ErrorContains(t, err, "illegal appPath")
		require.Nil(t, s.response)
	})
	t.Run("fails when script fails", func(t *testing.T) {
		s, err := NewMockParametersAnnouncementStream("./testdata/kustomize", "./testdata/kustomize", []string{"WRONG_ENV_VAR=oops"})
		require.NoError(t, err)
		err = service.GetParametersAnnouncement(s)
		require.ErrorContains(t, err, "error executing dynamic parameter output command")
		require.Nil(t, s.response)
	})
}

func TestService_CheckPluginConfiguration(t *testing.T) {
	type fixture struct {
		service *Service
	}
	setup := func(t *testing.T, opts ...pluginOpt) *fixture {
		t.Helper()
		cic := buildPluginConfig(opts...)
		s := NewService(*cic)
		return &fixture{
			service: s,
		}
	}
	t.Run("discovery is enabled when is configured", func(t *testing.T) {
		// given
		d := Discover{
			FileName: "kustomization.yaml",
		}
		f := setup(t, withDiscover(d))

		// when
		resp, err := f.service.CheckPluginConfiguration(context.Background(), &empty.Empty{})

		// then
		require.NoError(t, err)
		assert.True(t, resp.IsDiscoveryConfigured)
	})

	t.Run("discovery is disabled when is not configured", func(t *testing.T) {
		// given
		d := Discover{}
		f := setup(t, withDiscover(d))

		// when
		resp, err := f.service.CheckPluginConfiguration(context.Background(), &empty.Empty{})

		// then
		require.NoError(t, err)
		assert.False(t, resp.IsDiscoveryConfigured)
	})
}
