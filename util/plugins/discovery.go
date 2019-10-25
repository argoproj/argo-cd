package plugins

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/argoproj/pkg/exec"
	log "github.com/sirupsen/logrus"
	"github.com/xeipuuv/gojsonschema"

	"github.com/argoproj/argo-cd/util/config"
)

type Plugin struct {
	Path   string
	Schema string
	Loader gojsonschema.JSONLoader
}

func (p Plugin) Validate(spec string) (*gojsonschema.Result, error) {
	return gojsonschema.Validate(p.Loader, gojsonschema.NewStringLoader(spec))
}

var plugins = make(map[string]Plugin)

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
			path := filepath.Join(path, j.Name())
			output, err := exec.RunCommand(path, config.CmdOpts(), "schema")
			if err != nil {
				log.Fatal(err)
			}
			plugins[j.Name()] = Plugin{
				Path:   path,
				Schema: output,
				Loader: gojsonschema.NewStringLoader(output),
			}
		}
	}
	log.Infof("plugins loaded %v", Names())
}

func Names() []string {
	var names []string
	for name := range plugins {
		names = append(names, name)
	}
	return names
}

func Get(name string) Plugin {
	return plugins[name]
}
