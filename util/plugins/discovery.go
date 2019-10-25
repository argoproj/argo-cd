package plugins

import (
	"io/ioutil"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

var plugins = make(map[string]string)

func init() {
	for _, path := range []string{
		os.Getenv("ARGOCD_PLUGINS"),
		filepath.Join(os.Getenv("GOPATH"), "src/github.com/argoproj/argo-cd/plugins/bin"),
	} {
		infos, err := ioutil.ReadDir(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			log.Fatal(err)
		}
		for _, j := range infos {
			plugins[j.Name()] = filepath.Join(path, j.Name())
		}
	}
	log.Infof("loaded %v", plugins)
}

func Names() []string {
	var names []string
	for name := range plugins {
		names = append(names, name)
	}
	return names
}

func Path(name string) string {
	return plugins[name]
}
