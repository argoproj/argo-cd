package sandbox

import (
	"fmt"
	"strings"

	"github.com/landlock-lsm/go-landlock/landlock"
	llsyscall "github.com/landlock-lsm/go-landlock/landlock/syscall"
	log "github.com/sirupsen/logrus"
)

const LANDLOCK = "landlock"

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

var LandlockAllowArgs []string

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

func (m *Landlock) addAllowedPaths(entry LandlockAllowedPath) error {
	if len(entry.Paths) == 0 {
		return fmt.Errorf("no paths are given for access entry")
	}
	permittedAccess, err := m.createAccessFSSet(entry.Access)
	if err != nil {
		return fmt.Errorf("invalid access spec: %v", err)
	}
	log.Infof("* adding allowed paths: %s %q", entry.Access, entry.Paths)
	rule := landlock.PathAccess(permittedAccess, entry.Paths...)
	m.Rules = append(m.Rules, rule)
	return nil
}

func (m *Landlock) Init(sandboxConfig *ArgocdSandboxConfig, allowRulesStrs []string) error {
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
		err = m.addAllowedPaths(entry)
		if err != nil {
			return fmt.Errorf("Landlock sandbox cannot initialize: invalid allowedPaths entry #%d: %v", idx, err)
		}
	}

	for _, allowRuleStr := range allowRulesStrs {
		entry, err := parseAllowParam(allowRuleStr)
		if err != nil {
			return fmt.Errorf("Landlock sandbox cannot initialize: unparsable allow param %q: %v", allowRuleStr, err)
		}
		err = m.addAllowedPaths(entry)
		if err != nil {
			return fmt.Errorf("Landlock sandbox cannot initialize: invalid allow param %q: %v", allowRuleStr, err)
		}
	}
	return err
}

func (m *Landlock) Apply() error {
	log.Infof("  APPLYING config is: %x", m.Cfg)
	err := m.Cfg.RestrictPaths(m.Rules...)
	return err
}

func (m *Landlock) Name() string {
	return LANDLOCK
}

func (m *Landlock) GetConfig() string {
	return fmt.Sprintf("%v", m.Cfg)
}

// Parse parses a string in the format "fs:access_right1,access_right2,...:/absolute/path".
func parseAllowParam(input string) (LandlockAllowedPath, error) {
	if input == "" {
		return LandlockAllowedPath{}, fmt.Errorf("the rule is empty")
	}
	parts := strings.SplitN(input, ":", 3)
	if len(parts) != 3 {
		return LandlockAllowedPath{}, fmt.Errorf("expected format \"fs:<access_rights>:<path>\", got %d part(s)", len(parts))
	}

	prefix, rights, path := parts[0], parts[1], parts[2]

	if prefix != "fs" {
		return LandlockAllowedPath{}, fmt.Errorf("invalid prefix %q: expected \"fs\"", prefix)
	}

	rights = strings.TrimSpace(rights)

	if rights == "" {
		return LandlockAllowedPath{}, fmt.Errorf("empty access rights list")
	}

	if err := validatePath(path); err != nil {
		return LandlockAllowedPath{}, err
	}

	return LandlockAllowedPath{
		Access: rights,
		Paths:  []string{path},
	}, nil
}

func validatePath(p string) error {
	if p == "" {
		return fmt.Errorf("path is empty")
	}
	if !strings.HasPrefix(p, "/") {
		return fmt.Errorf("path must be absolute (start with /), got %q", p)
	}
	if strings.Contains(p, "\x00") {
		return fmt.Errorf("path contains null byte")
	}
	return nil
}
