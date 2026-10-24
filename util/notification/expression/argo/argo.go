package argo

import (
	"encoding/json"
	"strings"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/health"
	synccommon "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

const adaptiveCardVersion = "1.4"

type adaptiveCard struct {
	Type    string                `json:"type"`
	Version string                `json:"version"`
	Body    []adaptiveCardElement `json:"body"`
	Actions []adaptiveCardAction  `json:"actions,omitempty"`
}

type adaptiveCardElement struct {
	Type   string             `json:"type"`
	Text   string             `json:"text,omitempty"`
	Size   string             `json:"size,omitempty"`
	Weight string             `json:"weight,omitempty"`
	Wrap   bool               `json:"wrap,omitempty"`
	Facts  []adaptiveCardFact `json:"facts,omitempty"`
}

type adaptiveCardFact struct {
	Title string `json:"title"`
	Value string `json:"value"`
}

type adaptiveCardAction struct {
	Type  string `json:"type"`
	Title string `json:"title,omitempty"`
	URL   string `json:"url,omitempty"`
}

func NewExprs(app *unstructured.Unstructured) map[string]any {
	renderApplication := func(build func(*v1alpha1.Application) any) string {
		application, err := getApplication(app)
		if err != nil {
			panic(err)
		}

		return marshalJSON(build(application))
	}

	return map[string]any{
		"StatusCard": func(argocdURL string) string {
			return renderApplication(func(application *v1alpha1.Application) any {
				return buildStatusCard(application, argocdURL)
			})
		},
		"HeaderPanel": func() string {
			return renderApplication(func(application *v1alpha1.Application) any {
				return buildHeaderPanel(application)
			})
		},
		"SummaryPanel": func() string {
			return renderApplication(func(application *v1alpha1.Application) any {
				return buildSummaryPanel(application)
			})
		},
		"ApplicationLink": func(argocdURL string) string {
			application, err := getApplication(app)
			if err != nil {
				panic(err)
			}

			action, ok := buildApplicationLink(application, argocdURL)
			if !ok {
				return ""
			}

			return marshalJSON(action)
		},
	}
}

func getApplication(obj *unstructured.Unstructured) (*v1alpha1.Application, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	application := &v1alpha1.Application{}
	if err := json.Unmarshal(data, application); err != nil {
		return nil, err
	}

	return application, nil
}

func buildStatusCard(application *v1alpha1.Application, argocdURL string) adaptiveCard {
	card := adaptiveCard{
		Type:    "AdaptiveCard",
		Version: adaptiveCardVersion,
		Body: []adaptiveCardElement{
			buildHeaderPanel(application),
			buildSummaryPanel(application),
		},
	}

	if action, ok := buildApplicationLink(application, argocdURL); ok {
		card.Actions = []adaptiveCardAction{action}
	}

	return card
}

func buildHeaderPanel(application *v1alpha1.Application) adaptiveCardElement {
	title := "Argo CD application status"
	operationState := application.Status.OperationState

	switch {
	case operationState != nil &&
		(operationState.Phase == synccommon.OperationFailed ||
			operationState.Phase == synccommon.OperationError):
		title = "❌ Sync failed"
	case application.Status.Health.Status == health.HealthStatusDegraded:
		title = "🔴 Application degraded"
	case operationState != nil && operationState.Phase == synccommon.OperationRunning:
		title = "⏳ Deployment progressing"
	case application.Status.Health.Status == health.HealthStatusProgressing:
		title = "⏳ Deployment progressing"
	case operationState != nil &&
		operationState.Phase == synccommon.OperationSucceeded &&
		application.Status.Sync.Status == v1alpha1.SyncStatusCodeSynced &&
		application.Status.Health.Status == health.HealthStatusHealthy:
		title = "✅ Deployment succeeded"
	case application.Status.Sync.Status == v1alpha1.SyncStatusCodeOutOfSync:
		title = "🔄 Application out of sync"
	case application.Status.Sync.Status == v1alpha1.SyncStatusCodeUnknown:
		title = "⚠️ Sync status unknown"
	}

	return adaptiveCardElement{
		Type:   "TextBlock",
		Text:   title,
		Size:   "Large",
		Weight: "Bolder",
		Wrap:   true,
	}
}

func buildSummaryPanel(application *v1alpha1.Application) adaptiveCardElement {
	facts := []adaptiveCardFact{
		{
			Title: "Application",
			Value: application.Name,
		},
	}

	if namespace := application.Spec.Destination.Namespace; namespace != "" {
		facts = append(facts, adaptiveCardFact{
			Title: "Namespace",
			Value: namespace,
		})
	}

	if syncStatus := application.Status.Sync.Status; syncStatus != "" {
		facts = append(facts, adaptiveCardFact{
			Title: "Sync Status",
			Value: string(syncStatus),
		})
	}

	if healthStatus := application.Status.Health.Status; healthStatus != "" {
		facts = append(facts, adaptiveCardFact{
			Title: "Health Status",
			Value: string(healthStatus),
		})
	}

	revisions := application.Status.GetRevisions()
	if len(revisions) == 1 {
		facts = append(facts, adaptiveCardFact{
			Title: "Revision",
			Value: revisions[0],
		})
	} else if len(revisions) > 1 {
		facts = append(facts, adaptiveCardFact{
			Title: "Revisions",
			Value: strings.Join(revisions, ", "),
		})
	}

	return adaptiveCardElement{
		Type:  "FactSet",
		Facts: facts,
	}
}

func buildApplicationLink(application *v1alpha1.Application, argocdURL string) (adaptiveCardAction, bool) {
	argocdURL = strings.TrimRight(strings.TrimSpace(argocdURL), "/")
	if argocdURL == "" || application.Name == "" {
		return adaptiveCardAction{}, false
	}

	applicationName := application.Name
	if application.Namespace != "" {
		applicationName = application.Namespace + "/" + application.Name
	}

	return adaptiveCardAction{
		Type:  "Action.OpenUrl",
		Title: "View in Argo CD",
		URL:   argocdURL + "/applications/" + applicationName,
	}, true
}

func marshalJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}

	return string(data)
}
