package plugins

import (
	"io/ioutil"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

type Plugin struct {
	Path string
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
				plugins[j.Name()] = Plugin{Path: filepath.Join(path, j.Name())}
			}
		}
		log.Infof("plugins loaded %v", Names())
	}
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
