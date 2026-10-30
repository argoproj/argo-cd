package argo

import "fmt"

type ErrApplicationNotAllowedToUseProject struct {
	application string
	namespace   string
	project     string
}

func NewErrApplicationNotAllowedToUseProject(application, namespace, project string) error {
	return &ErrApplicationNotAllowedToUseProject{
		application: application,
		namespace:   namespace,
		project:     project,
	}
}

func (err *ErrApplicationNotAllowedToUseProject) Error() string {
	return fmt.Sprintf("application '%s' in namespace '%s' is not allowed to use project %s", err.application, err.namespace, err.project)
}
