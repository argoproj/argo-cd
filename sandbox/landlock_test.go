package sandbox

import (
	"os"
	"testing"

	testutil "github.com/argoproj/argo-cd/v3/util/test"
	"github.com/landlock-lsm/go-landlock/landlock"
	llsyscall "github.com/landlock-lsm/go-landlock/landlock/syscall"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateAccessFSSet(t *testing.T) {
	type testCase struct {
		spec   string
		result landlock.AccessFSSet
		errmsg string
	}
	testCases := []testCase{
		testCase{spec: "", result: 0},
		testCase{spec: "write_file", result: llsyscall.AccessFSWriteFile},
		testCase{spec: "write_file ,  read_dir", result: llsyscall.AccessFSWriteFile | llsyscall.AccessFSReadDir},
		testCase{spec: "nowrite", errmsg: "Invalid access specification given: \"nowrite\""},
	}
	ll := Landlock{}
	for _, testCase := range testCases {
		t.Run(testCase.spec, func(t *testing.T) {
			accessFSSet, err := ll.createAccessFSSet(testCase.spec)
			if testCase.errmsg == "" {
				assert.NoError(t, err)
				assert.Equal(t, testCase.result, accessFSSet)
			} else {
				assert.ErrorContains(t, err, testCase.errmsg)
			}
		})
	}
}

func TestInit(t *testing.T) {
	type testCase struct {
		name       string
		spec       ArgocdSandboxConfig
		allowRules []string
		errmsg     string
		cfg        *landlock.Config
	}
	newTestCase := func(name string, spec ArgocdSandboxConfig, allowRules []string, errmsg string, cfg *landlock.Config) testCase {
		result := testCase{}
		result.name = name
		result.spec = spec
		result.allowRules = allowRules
		result.errmsg = errmsg
		result.cfg = cfg
		return result
	}

	var rxAFSSet landlock.AccessFSSet
	rxAFSSet = llsyscall.AccessFSExecute | llsyscall.AccessFSReadFile | llsyscall.AccessFSReadDir
	rxCfg, err := landlock.NewConfig(rxAFSSet)
	assert.NoError(t, err)

	//rxCfg, _ :=landlock.NewConfig(llsyscall.AccessFSExecute | llsyscall.AccessFSReadFile |	llsyscall.AccessFSReadDir )
	testCases := []testCase{
		newTestCase("noconfig", ArgocdSandboxConfig{},
			[]string{}, "Landlock sandbox cannot initialize with no configuration given", nil),
		newTestCase("empty", ArgocdSandboxConfig{Landlock: &LandlockConfig{}}, []string{}, "", &landlock.Config{}),
		newTestCase("rx", ArgocdSandboxConfig{
			Landlock: &LandlockConfig{DefaultFSDeny: "execute, read_file, read_dir"},
		}, []string{}, "", rxCfg),
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ll := Landlock{}
			err := ll.Init(&testCase.spec, testCase.allowRules)
			if testCase.errmsg == "" {
				assert.NoError(t, err)

				if testCase.cfg == nil {
					assert.Nil(t, ll.Cfg)
				} else {
					assert.NotNil(t, ll.Cfg)
					assert.Equal(t, *testCase.cfg, *ll.Cfg)
				}
			} else {
				assert.ErrorContains(t, err, testCase.errmsg)
			}
		})
	}
}

func TestLandlockApply(t *testing.T) {
	testutil.RunInSubprocess(t, true, func() {
		ll := Landlock{}
		implConfig := ArgocdSandboxConfig{
			Landlock: &LandlockConfig{
				DefaultFSDeny: LANDLOCK_STD_RW,
			},
		}
		//fixTestCoverage(t, &implConfig.Landlock.AllowedPaths)
		err := ll.Init(&implConfig, []string{})
		require.NoError(t, err)
		err = ll.Apply()
		require.NoError(t, err)
		_, err = os.Open(".")
		require.ErrorContains(t, err, "permission denied")
	})
}

// func fixTestCoverage(t *testing.T, paths *[]LandlockAllowedPath) {
// 	t.Helper()
// }

// FIXME: breakes under gotestsum
// func TestLandlockConfigAccessRules(t *testing.T) {
// 	testutil.RunInSubprocess(t, true, func() {
// 		ll := Landlock{}
// 		implConfig := ArgocdSandboxConfig{
// 			Landlock: &LandlockConfig{
// 				DefaultFSDeny: "read_dir,read_file,write_file,make_dir",
// 			},
// 		}

// 		cwd, err := os.Getwd()
// 		require.NoError(t, err)

// 		allowedPaths := []LandlockAllowedPath{}
// 		allowedPath := LandlockAllowedPath{
// 			Access: "read_dir,read_file",
// 			Paths:  []string{},
// 		}
// 		allowedPath.Paths = append(allowedPath.Paths, cwd)
// 		allowedPaths = append(allowedPaths, allowedPath)
// 		fixTestCoverage(t, &allowedPaths)
// 		implConfig.Landlock.AllowedPaths = allowedPaths

// 		fmt.Printf("implConfig: %v", *implConfig.Landlock)
// 		err = ll.Init(&implConfig, []string{})
// 		require.NoError(t, err)
// 		err = ll.Apply()
// 		require.NoError(t, err)
// 		_, err = os.Open(cwd)
// 		require.NoError(t, err)

// 	})
// }

func TestMakeArgs(t *testing.T) {
	ll := Landlock{}
	tests := []struct {
		name         string
		runOpts      SandboxRunOpts
		expectedArgs []string
	}{
		{
			name:         "empty run opts",
			runOpts:      SandboxRunOpts{},
			expectedArgs: []string{},
		},
		{
			name: "ro dirs",
			runOpts: SandboxRunOpts{
				RODirs: []string{"/ro1", "/ro2"},
			},
			expectedArgs: []string{
				"--landlock-allow", "fs:read_dir,read_file:/ro1",
				"--landlock-allow", "fs:read_dir,read_file:/ro2",
			},
		},
		{
			name: "rorw dirs",
			runOpts: SandboxRunOpts{
				RODirs: []string{"/ro1", "/ro2"},
				RWDirs: []string{"/rw1", "/rw2"},
			},
			expectedArgs: []string{
				"--landlock-allow", "fs:read_dir,make_dir,read_file,write_file:/rw1",
				"--landlock-allow", "fs:read_dir,make_dir,read_file,write_file:/rw2",
				"--landlock-allow", "fs:read_dir,read_file:/ro1",
				"--landlock-allow", "fs:read_dir,read_file:/ro2",
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			args := ll.MakeArgs(&testCase.runOpts)
			assert.Equal(t, testCase.expectedArgs, args)
		})
	}
}

func TestParseValidDynamicRule(t *testing.T) {
	tests := []struct {
		name  string
		input string
		ops   string
		path  string
	}{
		{
			name:  "single operation",
			input: "fs:read_file:/some/file/path",
			ops:   "read_file",
			path:  "/some/file/path",
		},
		{
			name:  "multiple operations",
			input: "fs:read_file,read_dir:/some/file/path",
			ops:   "read_file,read_dir",
			path:  "/some/file/path",
		},
		{
			name:  "root path",
			input: "fs:read_dir:/",
			ops:   "read_dir",
			path:  "/",
		},
		{
			name:  "path with colon",
			input: "fs:read_file:/some/path:with:colons",
			ops:   "read_file",
			path:  "/some/path:with:colons",
		},
		{
			name:  "path with spaces",
			input: "fs:write_file:/home/user/my documents/file.txt",
			ops:   "write_file",
			path:  "/home/user/my documents/file.txt",
		},
		{
			name:  "path with unicode",
			input: "fs:read_file:/home/☺.txt",
			ops:   "read_file",
			path:  "/home/☺.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseAllowParam(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.ops, result.Access)
			assert.Equal(t, len(result.Paths), 1)
			assert.Equal(t, result.Paths[0], tt.path)
		})
	}
}

func TestParseInvalidDynamicRules(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty input", ""},
		{"missing parts", "fs:read_file"},
		{"wrong prefix", "zz:read_file:/path"},
		{"empty access rights list", "fs::/path"},
		{"empty path", "fs:read_file:"},
		{"relative path", "fs:read_file:relative/path"},
		{"null byte in path", "fs:read_file:/some/\x00path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseAllowParam(tt.input)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}

// FIXME: breakes under gotestsum
// func TestGenerateLandlockHelmConfig(t *testing.T) {
// 	testutil.RunInSubprocess(t, true, func() {
// 		ops := &ToolOpts{
// 			toolName:       "helm",
// 			isEnabled:      false,
// 			modulesList:    []string{LANDLOCK},
// 			configFilePath: "",
// 		}
// 		landlockCfg, err := GenerateDefaultLandlockConfig(ops)
// 		require.NoError(t, err)
// 		require.NotNil(t, landlockCfg)
// 		assert.True(t, landlockCfg.DefaultFSDeny != "")
// 		assert.True(t, len(landlockCfg.AllowedPaths) > 0)

// 		fixTestCoverage(t, &landlockCfg.AllowedPaths)

// 		kustomizeBinPath, err := exec.LookPath("kustomize") // not allowed, must fail
// 		require.NoError(t, err)

// 		toolBinPath, err := exec.LookPath(ops.toolName)
// 		require.NoError(t, err)

// 		cfg := &ArgocdSandboxConfig{
// 			Landlock: landlockCfg,
// 		}
// 		m := Landlock{}
// 		err = m.Init(cfg, nil)
// 		require.NoError(t, err)

// 		err = m.Apply()
// 		require.NoError(t, err)

// 		// FIXME: check process return code, output text
// 		cmd := exec.Command(toolBinPath)
// 		b, err := cmd.CombinedOutput()
// 		require.NoError(t, err)
// 		log.Infof("TOOL OUTPUT: %s", string(b))

// 		cmd = exec.Command(kustomizeBinPath)
// 		b, err = cmd.CombinedOutput()
// 		require.ErrorContains(t, err, kustomizeBinPath+": permission denied")
// 		//log.Infof("LS OUTPUT: %s", string(b))
// 	})

// }
