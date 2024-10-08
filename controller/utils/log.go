package utils

import (
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/sirupsen/logrus"
)

// GetAppLog returns a logrus entry with fields set for the given application.
func GetAppLog(app *v1alpha1.Application) *logrus.Entry {
	return logrus.WithFields(logrus.Fields{
		"application":        app.Name,
		"app-namespace":      app.Namespace,
		"app-qualified-name": app.QualifiedName(),
		"project":            app.Spec.Project,
	})
}
