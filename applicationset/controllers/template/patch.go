package template

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/util/strategicpatch"

	"github.com/argoproj/argo-cd/v2/applicationset/utils"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
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
