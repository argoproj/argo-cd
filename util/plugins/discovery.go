package plugins

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

func Discover() ([]string, error) {
	var plugins []string
	for _, path := range []string{
		os.Getenv("ARGOCD_PLUGINS"),
		filepath.Join(os.Getenv("GOPATH"), "src/github.com/argoproj/argo-cd/plugins/bin"),
	} {
		infos, err := ioutil.ReadDir(path)
		if err != nil {
			if os.IsNotExist(err){
				continue
			}
			return nil, err
		}
		for _, j := range infos {
			plugins = append(plugins, j.Name())
		}
	}
	return plugins, nil
}
