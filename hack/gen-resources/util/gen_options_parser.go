package util

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type SourceOpts struct {
	Strategy string `yaml:"strategy"`
}

type ApplicationOpts struct {
	Samples    int        `yaml:"samples"`
	SourceOpts SourceOpts `yaml:"source"`
}

type GenerateOpts struct {
	ApplicationOpts ApplicationOpts `yaml:"application"`
	GithubToken     string
	Namespace       string `yaml:"namespace"`
}

func Parse(opts *GenerateOpts, file string) error {
	fp, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	if e := yaml.Unmarshal(fp, &opts); e != nil {
		return e
	}

	return nil
}
