package plugin

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/cmpserver/apiclient"
	repoclient "github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/test"
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
	}
	setup := func(t *testing.T, opts ...pluginOpt) *fixture {
		t.Helper()
		cic := buildPluginConfig(opts...)
		path := filepath.Join(test.GetTestDir(t), "testdata", "kustomize")
		s := NewService(*cic)
		return &fixture{
			service: s,
			path:    path,
		}
	}
	t.Run("will match plugin by filename", func(t *testing.T) {
		// given
		d := Discover{
			FileName: "kustomization.yaml",
		}
		f := setup(t, withDiscover(d))

		// when
		match, err := f.service.matchRepository(context.Background(), f.path)

		// then
		assert.NoError(t, err)
		assert.True(t, match)
	})
	t.Run("will not match plugin by filename if file not found", func(t *testing.T) {
		// given
		d := Discover{
			FileName: "not_found.yaml",
		}
		f := setup(t, withDiscover(d))

		// when
		match, err := f.service.matchRepository(context.Background(), f.path)

		// then
		assert.NoError(t, err)
		assert.False(t, match)
	})
	t.Run("will not match a pattern with a syntax error", func(t *testing.T) {
		// given
		d := Discover{
			FileName: "[",
		}
		f := setup(t, withDiscover(d))

		// when
		_, err := f.service.matchRepository(context.Background(), f.path)

		// then
		assert.ErrorContains(t, err, "syntax error")
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
		match, err := f.service.matchRepository(context.Background(), f.path)

		// then
		assert.NoError(t, err)
		assert.True(t, match)
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
		match, err := f.service.matchRepository(context.Background(), f.path)

		// then
		assert.NoError(t, err)
		assert.False(t, match)
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
		_, err := f.service.matchRepository(context.Background(), f.path)

		// then
		assert.ErrorContains(t, err, "error finding glob match for pattern")
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
		match, err := f.service.matchRepository(context.Background(), f.path)

		// then
		assert.NoError(t, err)
		assert.True(t, match)
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
		match, err := f.service.matchRepository(context.Background(), f.path)

		// then
		assert.NoError(t, err)
		assert.False(t, match)
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
		match, err := f.service.matchRepository(context.Background(), f.path)

		// then
		assert.Error(t, err)
		assert.False(t, match)
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

		res1, err := service.generateManifest(context.Background(), "", nil)
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

		res, err := service.generateManifest(context.Background(), "", nil)
		assert.ErrorContains(t, err, "executable file not found")
		assert.Nil(t, res.Manifests)
	})
	t.Run("bad yaml output", func(t *testing.T) {
		service, err := newService(configFilePath)
		require.NoError(t, err)
		service.WithGenerateCommand(Command{Command: []string{"echo", "invalid yaml: }"}})

		res, err := service.generateManifest(context.Background(), "", nil)
		assert.ErrorContains(t, err, "failed to unmarshal manifest")
		assert.Nil(t, res.Manifests)
	})
}

func TestGenerateManifest_deadline_exceeded(t *testing.T) {
	configFilePath := "./testdata/kustomize/config"
	service, err := newService(configFilePath)
	require.NoError(t, err)

	expiredCtx, cancel := context.WithTimeout(context.Background(), time.Second * 0)
	defer cancel()
	_, err = service.generateManifest(expiredCtx, "", nil)
	assert.ErrorContains(t, err, "context deadline exceeded")
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
	assert.Error(t, err) // The command should time out, causing an error.
	assert.Less(t, after.Sub(before), 1*time.Second)
}

func TestRunCommandEmptyCommand(t *testing.T) {
	_, err := runCommand(context.Background(), Command{}, "", nil)
	assert.ErrorContains(t, err, "Command is empty")
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
	res, err := getParametersAnnouncement(context.Background(), "", *static, command)
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
	res, err := getParametersAnnouncement(context.Background(), "", *static, command)
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
	res, err := getParametersAnnouncement(context.Background(), "", *static, command)
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
	_, err := getParametersAnnouncement(context.Background(), "", []*repoclient.ParameterAnnouncement{}, command)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected end of JSON input")
}

func Test_getParametersAnnouncement_bad_command(t *testing.T) {
	command := Command{
		Command: []string{"exit"},
		Args:    []string{"1"},
	}
	_, err := getParametersAnnouncement(context.Background(), "", []*repoclient.ParameterAnnouncement{}, command)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error executing dynamic parameter output command")
}

func Test_getTempDirMustCleanup(t *testing.T) {
	tempDir := t.TempDir()

	// Induce a directory create error to verify error handling.
	err := os.Chmod(tempDir, 0000)
	require.NoError(t, err)
	_, _, err = getTempDirMustCleanup(path.Join(tempDir, "test"))
	assert.ErrorContains(t, err, "error creating temp dir")

	err = os.Chmod(tempDir, 0700)
	require.NoError(t, err)
	workDir, cleanup, err := getTempDirMustCleanup(tempDir)
	require.NoError(t, err)
	require.DirExists(t, workDir)

	// Induce a cleanup error to verify panic behavior.
	err = os.Chmod(tempDir, 0000)
	require.NoError(t, err)
	assert.Panics(t, func() {
		cleanup()
	}, "cleanup must panic to protect from directory traversal vulnerabilities")

	err = os.Chmod(tempDir, 0700)
	require.NoError(t, err)
	cleanup()
	assert.NoDirExists(t, workDir)
}

func TestService_Init(t *testing.T) {
	// Set up a base directory containing a test directory and a test file.
	tempDir := t.TempDir()
	workDir := path.Join(tempDir, "workDir")
	err := os.MkdirAll(workDir, 0700)
	require.NoError(t, err)
	testfile := path.Join(workDir, "testfile")
	file, err := os.Create(testfile)
	require.NoError(t, err)
	err = file.Close()
	require.NoError(t, err)

	// Make the base directory read-only so Init's cleanup fails.
	err = os.Chmod(tempDir, 0000)
	require.NoError(t, err)
	s := NewService(CMPServerInitConstants{PluginConfig: PluginConfig{}})
	err = s.Init(workDir)
	assert.ErrorContains(t, err, "error removing workdir", "Init must throw an error if it can't remove the work directory")

	// Make the base directory writable so Init's cleanup succeeds.
	err = os.Chmod(tempDir, 0700)
	require.NoError(t, err)
	err = s.Init(workDir)
	assert.NoError(t, err)
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
