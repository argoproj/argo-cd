package template

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/util/strategicpatch"

	jsonpatch "github.com/evanphx/json-patch"

	"github.com/argoproj/argo-cd/v3/applicationset/utils"
	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func applyTemplatePatch(app *appv1.Application, templatePatch string) (*appv1.Application, error) {
	appString, err := json.Marshal(app)
	if err != nil {
		return nil, fmt.Errorf("error while marhsalling Application %w", err)
	}

	convertedTemplatePatch, err := utils.ConvertYAMLToJSON(templatePatch)
	if err != nil {
		return nil, fmt.Errorf("error while converting template to json %q: %w", convertedTemplatePatch, err)
	}

	if err := json.Unmarshal([]byte(convertedTemplatePatch), &appv1.Application{}); err != nil {
		return nil, fmt.Errorf("invalid templatePatch %q: %w", convertedTemplatePatch, err)
	}

	data, err := strategicpatch.StrategicMergePatch(appString, []byte(convertedTemplatePatch), appv1.Application{})
	if err != nil {
		return nil, fmt.Errorf("error while applying templatePatch template to json %q: %w", convertedTemplatePatch, err)
	}

	finalApp := appv1.Application{}
	err = json.Unmarshal(data, &finalApp)
	if err != nil {
		return nil, fmt.Errorf("error while unmarhsalling patched application: %w", err)
	}

	// Prevent changes to the `project` field. This helps prevent malicious template patches
	finalApp.Spec.Project = app.Spec.Project

	return &finalApp, nil
}

func applyTemplateJSONPatch(app *appv1.Application, templateJSONPatch string) (*appv1.Application, error) {
	appString, err := json.Marshal(app)
	if err != nil {
		return nil, fmt.Errorf("error while marhsalling Application %w", err)
	}

	patch, err := jsonpatch.DecodePatch([]byte(templateJSONPatch))
	if err != nil {
		return nil, fmt.Errorf("error while decoding templateJSONPatch %q: %w", templateJSONPatch, err)
	}

	data, err := patch.Apply(appString)
	if err != nil {
		return nil, fmt.Errorf("error while applying templateJsonPatch %q: %w", templateJSONPatch, err)
	}

	finalApp := appv1.Application{}
	err = json.Unmarshal(data, &finalApp)
	if err != nil {
		return nil, fmt.Errorf("error while unmarhsalling patched application: %w", err)
	}

	// Prevent changes to the `project` field. This helps prevent malicious template patches
	finalApp.Spec.Project = app.Spec.Project

	return &finalApp, nil
}
