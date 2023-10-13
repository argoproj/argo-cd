package util

import (
	"gopkg.in/yaml.v2"
	"os"
)

type SourceOpts struct {
	Strategy string `yaml:"strategy"`
}

type DestinationOpts struct {
	Strategy string `yaml:"strategy"`
}

type ApplicationOpts struct {
	SourceOpts      SourceOpts      `yaml:"source"`
	DestinationOpts DestinationOpts `yaml:"destination"`
	Samples         int             `yaml:"samples"`
}

type RepositoryOpts struct {
	Samples int `yaml:"samples"`
}

type ProjectOpts struct {
	Samples int `yaml:"samples"`
}

type ClusterOpts struct {
	NamespacePrefix      string `yaml:"namespacePrefix"`
	ValuesFilePath       string `yaml:"valuesFilePath"`
	DestinationNamespace string `yaml:"destinationNamespace"`
	ClusterNamePrefix    string `yaml:"clusterNamePrefix"`
	Samples              int    `yaml:"samples"`
	Concurrency          int    `yaml:"parallel"`
}

type GenerateOpts struct {
	ApplicationOpts ApplicationOpts `yaml:"application"`
	GithubToken     string
	Namespace       string         `yaml:"namespace"`
	ClusterOpts     ClusterOpts    `yaml:"cluster"`
	RepositoryOpts  RepositoryOpts `yaml:"repository"`
	ProjectOpts     ProjectOpts    `yaml:"project"`
}

func setDefaults(opts *GenerateOpts) {
	if opts.ClusterOpts.Concurrency == 0 {
		opts.ClusterOpts.Concurrency = 2
	}
}

func Parse(opts *GenerateOpts, file string) error {
	fp, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	if e := yaml.Unmarshal(fp, &opts); e != nil {
		return e
	}

	setDefaults(opts)

	return nil
}
