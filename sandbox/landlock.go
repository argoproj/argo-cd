package sandbox

import (
	"fmt"
	"strings"

	"github.com/landlock-lsm/go-landlock/landlock"
	llsyscall "github.com/landlock-lsm/go-landlock/landlock/syscall"
	log "github.com/sirupsen/logrus"
)

type LandlockAllowedPath struct {
	Paths  []string `json:"paths"`
	Access string   `json:"access"`
}

type LandlockConfig struct {
	DefaultFSDeny string                `json:"defaultFSDeny"`
	AllowedPaths  []LandlockAllowedPath `json:"allowedPaths,omitempty"`
}

type Landlock struct {
	Cfg   *landlock.Config
	Rules []landlock.Rule
}

// strings according to bit index in the bitmap
var accessFSNames = map[string]landlock.AccessFSSet{
	"execute":     llsyscall.AccessFSExecute,
	"write_file":  llsyscall.AccessFSWriteFile,
	"read_file":   llsyscall.AccessFSReadFile,
	"read_dir":    llsyscall.AccessFSReadDir,
	"remove_dir":  llsyscall.AccessFSRemoveDir,
	"remove_file": llsyscall.AccessFSRemoveFile,
	"make_char":   llsyscall.AccessFSMakeChar,
	"make_dir":    llsyscall.AccessFSMakeDir,
	"make_reg":    llsyscall.AccessFSMakeReg,
	"make_sock":   llsyscall.AccessFSMakeSock,
	"make_fifo":   llsyscall.AccessFSMakeFifo,
	"make_block":  llsyscall.AccessFSMakeBlock,
	"make_sym":    llsyscall.AccessFSMakeSym,
	"refer":       llsyscall.AccessFSRefer,
	"truncate":    llsyscall.AccessFSTruncate,
	"ioctl_dev":   llsyscall.AccessFSIoctlDev,
}

const fsAccessAll = llsyscall.AccessFSExecute |
	llsyscall.AccessFSWriteFile |
	llsyscall.AccessFSReadFile |
	llsyscall.AccessFSReadDir |
	llsyscall.AccessFSRemoveDir |
	llsyscall.AccessFSRemoveFile |
	llsyscall.AccessFSMakeChar |
	llsyscall.AccessFSMakeDir |
	llsyscall.AccessFSMakeReg |
	llsyscall.AccessFSMakeSock |
	llsyscall.AccessFSMakeFifo |
	llsyscall.AccessFSMakeBlock |
	llsyscall.AccessFSMakeSym |
	llsyscall.AccessFSRefer |
	llsyscall.AccessFSTruncate |
	llsyscall.AccessFSIoctlDev

// func ValidateLandlockConfig(config LandlockConfig) error {

// }

func (m *Landlock) createAccessFSSet(spec string) (landlock.AccessFSSet, error) {
	var result landlock.AccessFSSet
	specvals := strings.Split(spec, ",")
	for _, specval := range specvals {
		specval = strings.TrimSpace(specval)
		if specval == "*" {
			result = fsAccessAll
			break
		}
		if specval == "" {
			continue
		}
		accessFlag, ok := accessFSNames[specval]
		if !ok {
			return 0, fmt.Errorf("Invalid access specification given: %q", specval)
		}
		result |= accessFlag
	}
	return result, nil
}

func (m *Landlock) Init(sandboxConfig *ArgocdSandboxConfig) error {
	implConfig := sandboxConfig.Landlock
	if implConfig == nil {
		return fmt.Errorf("Landlock sandbox cannot initialize with no configuration given")
	}
	accessFSSet, err := m.createAccessFSSet(implConfig.DefaultFSDeny)
	m.Cfg, err = landlock.NewConfig(accessFSSet)
	if err != nil {
		return fmt.Errorf("Landlock sandbox cannot initialize configuration: %v", err)
	}
	for idx, entry := range implConfig.AllowedPaths {
		if len(entry.Paths) == 0 {
			return fmt.Errorf("Landlock sandbox cannot initialize: no paths are given for access entry #%d in configuration", idx)
		}
		permittedAccess, err := m.createAccessFSSet(entry.Access)
		if err != nil {
			return fmt.Errorf("Landlock sandbox cannot initialize: invalid access spec in access entry #%d in configuration: %v", idx, err)
		}
		rule := landlock.PathAccess(permittedAccess, entry.Paths...)

		m.Rules = append(m.Rules, rule)
	}
	//m.Cfg.RestrictPaths(rules...)
	return err
}

func (m *Landlock) Apply() error {
	log.Infof("  config is: %x", m.Cfg)
	err := m.Cfg.RestrictPaths(m.Rules...)
	return err
}

func (m *Landlock) Name() string {
	return "landlock"
}

func (m *Landlock) GetConfig() string {
	return fmt.Sprintf("%v", m.Cfg)
}
