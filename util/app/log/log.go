package log

import (
	log "github.com/sirupsen/logrus"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func GetAppLogFields(app *appv1.Application) log.Fields {
	return log.Fields{
		"application":   app.Name,
		"app-namespace": app.Namespace,
		"project":       app.Spec.Project,
	}
}

func GetAppSetLogFields(appset *appv1.ApplicationSet) log.Fields {
	return log.Fields{
		"applicationSet":   appset.Name,
		"appSet-namespace": appset.Namespace,
		"project":          appset.Spec.Template.Spec.Project,
	}
}
