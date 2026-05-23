package sandbox

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/landlock-lsm/go-landlock/landlock"
	llsyscall "github.com/landlock-lsm/go-landlock/landlock/syscall"
	log "github.com/sirupsen/logrus"
)

const (
	LANDLOCK             = "landlock"
	LANDLOCK_STD_RW      = "read_dir,make_dir,read_file,write_file"
	LANDLOCK_STD_FILE_RW = "read_file,write_file"
	LANDLOCK_STD_RO      = "read_dir,read_file"
	LANDLOCK_STD_FILE_RO = "read_file"
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
	specvals := strings.SplitSeq(spec, ",")
	for specval := range specvals {
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
			return 0, fmt.Errorf("invalid access specification given: %q", specval)
		}
		result |= accessFlag
	}
	return result, nil
}

func (m *Landlock) addAllowedPaths(entry LandlockAllowedPath) error {
	if len(entry.Paths) == 0 {
		return errors.New("no paths are given for access entry")
	}
	permittedAccess, err := m.createAccessFSSet(entry.Access)
	if err != nil {
		return fmt.Errorf("invalid access spec: %w", err)
	}
	log.Infof("* adding allowed paths: %s %q", entry.Access, entry.Paths)
	rule := landlock.PathAccess(permittedAccess, entry.Paths...)
	m.Rules = append(m.Rules, rule)
	return nil
}

func (m *Landlock) Init(sandboxConfig *ArgocdSandboxConfig, allowRulesStrs []string) error {
	implConfig := sandboxConfig.Landlock
	if implConfig == nil {
		return errors.New("Landlock sandbox cannot initialize with no configuration given")
	}
	accessFSSet, err := m.createAccessFSSet(implConfig.DefaultFSDeny)
	if err != nil {
		return fmt.Errorf("Landlock sandbox cannot create default filesystem deny set: %w", err)
	}
	m.Cfg, err = landlock.NewConfig(accessFSSet)
	if err != nil {
		return fmt.Errorf("Landlock sandbox cannot initialize configuration: %w", err)
	}
	for idx, entry := range implConfig.AllowedPaths {
		err = m.addAllowedPaths(entry)
		if err != nil {
			return fmt.Errorf("Landlock sandbox cannot initialize: invalid allowedPaths entry #%d: %w", idx, err)
		}
	}

	for _, allowRuleStr := range allowRulesStrs {
		entry, err := parseAllowParam(allowRuleStr)
		if err != nil {
			return fmt.Errorf("Landlock sandbox cannot initialize: unparsable allow param %q: %w", allowRuleStr, err)
		}
		err = m.addAllowedPaths(entry)
		if err != nil {
			return fmt.Errorf("Landlock sandbox cannot initialize: invalid allow param %q: %w", allowRuleStr, err)
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
	return m.Cfg.String()
}

func (m *Landlock) makeFSArgs(accessSpec string, paths []string) []string {
	result := []string{}
	prefix := "fs:" + accessSpec + ":"
	for _, path := range paths {
		if !filepath.IsAbs(path) {
			// FIXME: is this the right place for this check
			path, _ = filepath.Abs(path)
		}
		result = append(result, "--landlock-allow", prefix+path)
	}
	return result
}

func (m *Landlock) MakeArgs(runOpts *SandboxRunOpts) []string {
	result := []string{}
	if runOpts != nil {
		result = append(result, m.makeFSArgs(LANDLOCK_STD_RW, runOpts.RWDirs)...)
		result = append(result, m.makeFSArgs(LANDLOCK_STD_RO, runOpts.RODirs)...)
		result = append(result, m.makeFSArgs(LANDLOCK_STD_FILE_RO, runOpts.ROFiles)...)
	}
	return result
}

// Parse parses a string in the format "fs:access_right1,access_right2,...:/absolute/path".
func parseAllowParam(input string) (LandlockAllowedPath, error) {
	if input == "" {
		return LandlockAllowedPath{}, errors.New("the rule is empty")
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
		return LandlockAllowedPath{}, errors.New("empty access rights list")
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
		return errors.New("path is empty")
	}
	if !strings.HasPrefix(p, "/") {
		return fmt.Errorf("path must be absolute (start with /), got %q", p)
	}
	if strings.Contains(p, "\x00") {
		return errors.New("path contains null byte")
	}
	return nil
}

func GenerateDefaultLandlockConfig(ops *ToolOpts) (*LandlockConfig, error) {
	result := LandlockConfig{}
	result.DefaultFSDeny = "read_dir,read_file,write_file,make_dir,execute"
	binPath, err := exec.LookPath(ops.toolName)
	if err != nil {
		return nil, fmt.Errorf("command %q not found in PATH: %w", ops.toolName, err)
	}
	binPath, err = filepath.Abs(binPath)
	if err != nil {
		return nil, fmt.Errorf("command %q not found in PATH: %w", ops.toolName, err)
	}
	readPaths := []string{"/dev/null", "/dev/urandom"}
	if ops.IsNetworkEnabled {
		readPaths = append(readPaths, "/etc/resolv.conf", "/etc/nsswitch.conf", "/etc/hosts",
			// FIXME: select correct at runtime!  the
			// possibilities are:
			// /etc/ssl/certs/ca-certificates.crt
			// /etc/pki/tls/certs/ca-bundle.crt
			// /etc/ssl/ca-bundle.pem
			// /etc/pki/tls/cacert.pem
			// /etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem
			// /etc/ssl/cert.pem
			// support env vars:
			"/etc/ssl/certs/ca-certificates.crt",
			"/dev/urandom",
			/*"/etc/services", "/etc/protocols"*/)
	}
	result.AllowedPaths = append(result.AllowedPaths,
		LandlockAllowedPath{
			Access: "read_file,execute",
			Paths:  []string{binPath},
		},
		LandlockAllowedPath{
			Access: "read_file",
			Paths:  readPaths,
		},
	)

	return &result, nil
}
