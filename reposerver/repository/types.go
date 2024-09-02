package repository

// Chart see: https://helm.sh/docs/topics/charts/ for more details
type Chart struct {
	Description string       `yaml:"description,omitempty"`
	Home        string       `yaml:"home,omitempty"`
	Maintainers []Maintainer `yaml:"maintainers,omitempty"`
}

type Maintainer struct {
	Name  string `yaml:"name,omitempty"`
	Email string `yaml:"email,omitempty"`
	Url   string `yaml:"url,omitempty"`
}
