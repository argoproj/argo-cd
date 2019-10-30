package plugins

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/argoproj/pkg/exec"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/util/config"
)

type Plugin struct {
	Name string
	Path string
}

func (p Plugin) Discover(path string) ([]string, error) {
	output, err := exec.RunCommand(p.Path, config.CmdOpts(), "discover", path)
	if err != nil {
		return nil, err
	}
	split := strings.Split(output, "\n")
	if len(split) == 1 && split[0] == "" {
		return nil, nil
	}
	return split, nil
}

func (p Plugin) Version() (string, error) {
	output, err := exec.RunCommand(p.Path, config.CmdOpts(), "version")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

var plugins map[string]Plugin

func lazyInit() {
	if plugins == nil {
		plugins = make(map[string]Plugin)
		for _, path := range []string{
			os.Getenv("ARGOCD_PLUGINS"),
			filepath.Join(os.Getenv("GOPATH"), "src/github.com/argoproj/argo-cd/plugins/bin/"),
		} {
			infos, err := ioutil.ReadDir(path)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				log.Warn(err)
			}
			for _, j := range infos {
				plugins[j.Name()] = Plugin{Name: j.Name(), Path: filepath.Join(path, j.Name())}
			}
		}
		log.Infof("plugins loaded %v", Names())
	}
}
func Plugins() map[string]Plugin {
	lazyInit()
	return plugins
}

func Names() []string {
	lazyInit()
	var names []string
	for name := range plugins {
		names = append(names, name)
	}
	return names
}

func Get(name string) Plugin {
	lazyInit()
	return plugins[name]
}
