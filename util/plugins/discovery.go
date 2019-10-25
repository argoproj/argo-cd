package plugins

import (
	"io/ioutil"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

var plugins []string

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
			plugins = append(plugins, j.Name())
		}
	}
	log.Infof("loaded %v", plugins)
}

func Discover() []string {
	return plugins
}
