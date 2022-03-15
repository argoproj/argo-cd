package util

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type SourceOpts struct {
	Strategy string `yaml:"strategy"`
}

type DestinationOpts struct {
	Strategy string `yaml:"strategy"`
}

type ApplicationOpts struct {
	Samples         int             `yaml:"samples"`
	SourceOpts      SourceOpts      `yaml:"source"`
	DestinationOpts DestinationOpts `yaml:"destination"`
}

type RepositoryOpts struct {
	Samples int `yaml:"samples"`
}

type ProjectOpts struct {
	Samples int `yaml:"samples"`
}

type ClusterOpts struct {
	Samples              int    `yaml:"samples"`
	NamespacePrefix      string `yaml:"namespacePrefix"`
	ValuesFilePath       string `yaml:"valuesFilePath"`
	DestinationNamespace string `yaml:"destinationNamespace"`
	ClusterNamePrefix    string `yaml:"clusterNamePrefix"`
}

type GenerateOpts struct {
	ApplicationOpts ApplicationOpts `yaml:"application"`
	ClusterOpts     ClusterOpts     `yaml:"cluster"`
	RepositoryOpts  RepositoryOpts  `yaml:"repository"`
	ProjectOpts     ProjectOpts     `yaml:"project"`
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
