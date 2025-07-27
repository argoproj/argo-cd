package commands

import (
	"context"
	"testing"

	argoappv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestValidateManifests(t *testing.T) {
	tests := []struct {
		name           string
		items          []objKeyLiveTarget
		app            *argoappv1.Application
		expectedErrors int
		errorContains  string
	}{
		{
			name: "valid deployment",
			items: []objKeyLiveTarget{
				{
					key: kube.ResourceKey{
						Group:     "apps",
						Kind:      "Deployment",
						Namespace: "default",
						Name:      "test-deployment",
					},
					target: &unstructured.Unstructured{
						Object: map[string]any{
							"apiVersion": "apps/v1",
							"kind":       "Deployment",
							"metadata": map[string]any{
								"name":      "test-deployment",
								"namespace": "default",
							},
							"spec": map[string]any{
								"replicas": 3,
								"selector": map[string]any{
									"matchLabels": map[string]any{
										"app": "test",
									},
								},
								"template": map[string]any{
									"metadata": map[string]any{
										"labels": map[string]any{
											"app": "test",
										},
									},
									"spec": map[string]any{
										"containers": []any{
											map[string]any{
												"name":  "nginx",
												"image": "nginx:latest",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			app: &argoappv1.Application{
				Spec: argoappv1.ApplicationSpec{
					Destination: argoappv1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "default",
					},
				},
			},
			expectedErrors: 0,
		},
		{
			name: "invalid deployment - replicas as string",
			items: []objKeyLiveTarget{
				{
					key: kube.ResourceKey{
						Group:     "apps",
						Kind:      "Deployment",
						Namespace: "default",
						Name:      "invalid-deployment",
					},
					target: &unstructured.Unstructured{
						Object: map[string]any{
							"apiVersion": "apps/v1",
							"kind":       "Deployment",
							"metadata": map[string]any{
								"name":      "invalid-deployment",
								"namespace": "default",
							},
							"spec": map[string]any{
								"replicas": "foo", // Invalid: should be number
								"selector": map[string]any{
									"matchLabels": map[string]any{
										"app": "test",
									},
								},
								"template": map[string]any{
									"metadata": map[string]any{
										"labels": map[string]any{
											"app": "test",
										},
									},
									"spec": map[string]any{
										"containers": []any{
											map[string]any{
												"name":  "nginx",
												"image": "nginx:latest",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			app: &argoappv1.Application{
				Spec: argoappv1.ApplicationSpec{
					Destination: argoappv1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "default",
					},
				},
			},
			expectedErrors: 1,
			errorContains:  "replicas",
		},
		{
			name: "skip deleted items",
			items: []objKeyLiveTarget{
				{
					key: kube.ResourceKey{
						Group:     "v1",
						Kind:      "Service",
						Namespace: "default",
						Name:      "deleted-service",
					},
					live:   &unstructured.Unstructured{},
					target: nil,
				},
			},
			app: &argoappv1.Application{
				Spec: argoappv1.ApplicationSpec{
					Destination: argoappv1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "default",
					},
				},
			},
			expectedErrors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			errors := validateManifests(ctx, tt.items, tt.app)
			assert.NotNil(t, errors)
		})
	}
}
