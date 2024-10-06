package util

import (
	"fmt"
	"os"

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
	Concurrency          int    `yaml:"parallel"`
}

type GenerateOpts struct {
	ApplicationOpts ApplicationOpts `yaml:"application"`
	ClusterOpts     ClusterOpts     `yaml:"cluster"`
	RepositoryOpts  RepositoryOpts  `yaml:"repository"`
	ProjectOpts     ProjectOpts     `yaml:"project"`
	GithubToken     string
	Namespace       string `yaml:"namespace"`
}

func setDefaults(opts *GenerateOpts) {
	if opts.ClusterOpts.Concurrency == 0 {
		opts.ClusterOpts.Concurrency = 2
	}
}

func Parse(opts *GenerateOpts, file string) error {
	fp, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("error reading the template file: %s : %w", file, err)
	}

	if e := yaml.Unmarshal(fp, &opts); e != nil {
		return e
	}

	setDefaults(opts)

	return nil
}
