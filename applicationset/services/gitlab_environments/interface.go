package gitlab_environments

import (
	"context"
)

type EnvironmentService interface {
	// List gets a list of environments.
	List(ctx context.Context) ([]*Environment, error)
}

type Environment struct {
	ID          int
	Name        string
	Slug        string
	State       string
	Tier        string
	ExternalURL string
	Project     string
}
