package template

import (
	"fmt"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/applicationset/generators"
	"github.com/argoproj/argo-cd/v3/applicationset/utils"

	argov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func GenerateApplications(logCtx *log.Entry, applicationSetInfo argov1alpha1.ApplicationSet, g map[string]generators.Generator, renderer utils.Renderer, client client.Client) ([]argov1alpha1.Application, argov1alpha1.ApplicationSetReasonType, error) {
	var res []argov1alpha1.Application

	var firstError error
	var applicationSetReason argov1alpha1.ApplicationSetReasonType

	for _, requestedGenerator := range applicationSetInfo.Spec.Generators {
		t, err := generators.Transform(requestedGenerator, g, applicationSetInfo.Spec.Template, &applicationSetInfo, map[string]any{}, client)
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
			tmplApplication := GetTempApplication(a.Template)

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

				if applicationSetInfo.Spec.TemplatePatch != nil {
					patchedApplication, err := renderTemplatePatch(renderer, app, applicationSetInfo, p)
					if err != nil {
						log.WithError(err).WithField("params", a.Params).WithField("generator", requestedGenerator).
							Error("error generating application from params")

						if firstError == nil {
							firstError = err
							applicationSetReason = argov1alpha1.ApplicationSetReasonRenderTemplateParamsError
						}
						continue
					}

					app = patchedApplication
				}

				if applicationSetInfo.Spec.TemplateJSONPatch != nil {
					patchedApplication, err := renderTemplateJSONPatch(renderer, app, applicationSetInfo, p)
					if err != nil {
						log.WithError(err).WithField("params", a.Params).WithField("generator", requestedGenerator).
							Error("error generating application from params")

						if firstError == nil {
							firstError = err
							applicationSetReason = argov1alpha1.ApplicationSetReasonRenderTemplateParamsError
						}
						continue
					}

					app = patchedApplication
				}

				// The app's namespace must be the same as the AppSet's namespace to preserve the appsets-in-any-namespace
				// security boundary.
				app.Namespace = applicationSetInfo.Namespace
				res = append(res, *app)
			}
		}
		if log.IsLevelEnabled(log.DebugLevel) {
			logCtx.WithField("generator", requestedGenerator).Debugf("apps from generator: %+v", res)
		} else {
			logCtx.Infof("generated %d applications", len(res))
		}
	}

	return res, applicationSetReason, firstError
}

func renderTemplatePatch(r utils.Renderer, app *argov1alpha1.Application, applicationSetInfo argov1alpha1.ApplicationSet, params map[string]any) (*argov1alpha1.Application, error) {
	replacedTemplate, err := r.Replace(*applicationSetInfo.Spec.TemplatePatch, params, applicationSetInfo.Spec.GoTemplate, applicationSetInfo.Spec.GoTemplateOptions)
	if err != nil {
		return nil, fmt.Errorf("error replacing values in templatePatch: %w", err)
	}

	return applyTemplatePatch(app, replacedTemplate)
}

func renderTemplateJSONPatch(r utils.Renderer, app *argov1alpha1.Application, applicationSetInfo argov1alpha1.ApplicationSet, params map[string]any) (*argov1alpha1.Application, error) {
	replacedTemplate, err := r.Replace(*applicationSetInfo.Spec.TemplateJSONPatch, params, applicationSetInfo.Spec.GoTemplate, applicationSetInfo.Spec.GoTemplateOptions)
	if err != nil {
		return nil, fmt.Errorf("error replacing values in templateJSONPatch: %w", err)
	}

	// If the templateJSONPatch does not appear to be a
	// json array after rendendering do not try to apply
	// json+patches. This deals with templateJSONPatchs
	// that are not rendered from templeting.
	if utils.IsJSONArray(replacedTemplate) {
		return applyTemplateJSONPatch(app, strings.TrimSpace(replacedTemplate))
	}
	return app, nil
}

func GetTempApplication(applicationSetTemplate argov1alpha1.ApplicationSetTemplate) *argov1alpha1.Application {
	var tmplApplication argov1alpha1.Application
	tmplApplication.Annotations = applicationSetTemplate.Annotations
	tmplApplication.Labels = applicationSetTemplate.Labels
	tmplApplication.Namespace = applicationSetTemplate.Namespace
	tmplApplication.Name = applicationSetTemplate.Name
	tmplApplication.Spec = applicationSetTemplate.Spec
	tmplApplication.Finalizers = applicationSetTemplate.Finalizers

	return &tmplApplication
}
