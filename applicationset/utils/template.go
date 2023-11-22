package utils

import (
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v2/applicationset/generators"

	argov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func GenerateApplications(logCtx *log.Entry, applicationSetInfo argov1alpha1.ApplicationSet, g map[string]generators.Generator, renderer Renderer) ([]argov1alpha1.Application, argov1alpha1.ApplicationSetReasonType, error) {
	var res []argov1alpha1.Application

	var firstError error
	var applicationSetReason argov1alpha1.ApplicationSetReasonType

	for _, requestedGenerator := range applicationSetInfo.Spec.Generators {
		t, err := generators.Transform(requestedGenerator, g, applicationSetInfo.Spec.Template, &applicationSetInfo, map[string]interface{}{})
		if err != nil {
			logCtx.WithError(err).WithField("generator", requestedGenerator).
				Error("error generating application from params")
			if firstError == nil {
				firstError = err
				applicationSetReason = argov1alpha1.ApplicationSetReasonApplicationParamsGenerationError
			}
			continue
		}

		for _, a := range t {
			tmplApplication := getTempApplication(a.Template)

			for _, p := range a.Params {
				app, err := renderer.RenderTemplateParams(tmplApplication, applicationSetInfo.Spec.SyncPolicy, p, applicationSetInfo.Spec.GoTemplate, applicationSetInfo.Spec.GoTemplateOptions)
				if err != nil {
					logCtx.WithError(err).WithField("params", a.Params).WithField("generator", requestedGenerator).
						Error("error generating application from params")

					if firstError == nil {
						firstError = err
						applicationSetReason = argov1alpha1.ApplicationSetReasonRenderTemplateParamsError
					}
					continue
				}
				res = append(res, *app)
			}
		}

		logCtx.WithField("generator", requestedGenerator).Infof("generated %d applications", len(res))
		logCtx.WithField("generator", requestedGenerator).Debugf("apps from generator: %+v", res)
	}

	return res, applicationSetReason, firstError
}

func getTempApplication(applicationSetTemplate argov1alpha1.ApplicationSetTemplate) *argov1alpha1.Application {
	var tmplApplication argov1alpha1.Application
	tmplApplication.Annotations = applicationSetTemplate.Annotations
	tmplApplication.Labels = applicationSetTemplate.Labels
	tmplApplication.Namespace = applicationSetTemplate.Namespace
	tmplApplication.Name = applicationSetTemplate.Name
	tmplApplication.Spec = applicationSetTemplate.Spec
	tmplApplication.Finalizers = applicationSetTemplate.Finalizers

	return &tmplApplication
}
