package sandbox

import (
	"fmt"
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
		name   string
		spec   ArgocdSandboxConfig
		errmsg string
		cfg    *landlock.Config
	}
	newTestCase := func(name string, spec ArgocdSandboxConfig, errmsg string, cfg *landlock.Config) testCase {
		result := testCase{}
		result.name = name
		result.spec = spec
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
		newTestCase("noconfig", ArgocdSandboxConfig{}, "Landlock sandbox cannot initialize with no configuration given", nil),
		newTestCase("empty", ArgocdSandboxConfig{Landlock: &LandlockConfig{}}, "", &landlock.Config{}),
		newTestCase("rx", ArgocdSandboxConfig{Landlock: &LandlockConfig{DefaultFSDeny: "execute, read_file, read_dir"}}, "", rxCfg),
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ll := Landlock{}
			err := ll.Init(&testCase.spec)
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

// func TestInitLandlockDomain(t *testing.T) {
// 	runInSubprocess(t, func() {
// 		err := InitLandlockDomain()
// 		require.NoError(t, err)
// 	})
// }

func TestLandlockApply(t *testing.T) {
	testutil.RunInSubprocess(t, func() {
		ll := Landlock{}
		implConfig := ArgocdSandboxConfig{
			Landlock: &LandlockConfig{
				DefaultFSDeny: "read_dir",
			},
		}
		ll.Init(&implConfig)
		err := ll.Apply()
		require.NoError(t, err)
		_, err = os.Open(".")
		require.ErrorContains(t, err, "permission denied")
	})
}

func TestLandlockConfigAccessRules(t *testing.T) {
	testutil.RunInSubprocess(t, func() {
		ll := Landlock{}
		implConfig := ArgocdSandboxConfig{
			Landlock: &LandlockConfig{
				DefaultFSDeny: "read_dir,read_file",
			},
		}

		cwd, err := os.Getwd()
		allowedPaths := []LandlockAllowedPath{}
		allowedPath := LandlockAllowedPath{
			Access: "read_dir,read_file",
			Paths:  []string{},
		}
		allowedPath.Paths = append(allowedPath.Paths, cwd)
		require.NoError(t, err)
		allowedPaths = append(allowedPaths, allowedPath)
		implConfig.Landlock.AllowedPaths = allowedPaths
		fmt.Printf("implConfig: %v", *implConfig.Landlock)
		err = ll.Init(&implConfig)
		require.NoError(t, err)
		err = ll.Apply()
		require.NoError(t, err)
		_, err = os.Open(cwd)
		require.NoError(t, err)

	})
}
