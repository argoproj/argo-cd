package controllers

import (
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/argoproj/argo-cd/v2/applicationset/generators"
	argov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func TestClusterEventHandler(t *testing.T) {

	scheme := runtime.NewScheme()
	err := argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	err = argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	tests := []struct {
		name             string
		items            []argov1alpha1.ApplicationSet
		secret           corev1.Secret
		expectedRequests []ctrl.Request
	}{
		{
			name:  "no application sets should mean no requests",
			items: []argov1alpha1.ApplicationSet{},
			secret: corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "argocd",
					Name:      "my-secret",
					Labels: map[string]string{
						generators.ArgoCDSecretTypeLabel: generators.ArgoCDSecretTypeCluster,
					},
				},
			},
			expectedRequests: []reconcile.Request{},
		},
		{
			name: "a cluster generator should produce a request",
			items: []argov1alpha1.ApplicationSet{
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-app-set",
						Namespace: "argocd",
					},
					Spec: argov1alpha1.ApplicationSetSpec{
						Generators: []argov1alpha1.ApplicationSetGenerator{
							{
								Clusters: &argov1alpha1.ClusterGenerator{},
							},
						},
					},
				},
			},
			secret: corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "argocd",
					Name:      "my-secret",
					Labels: map[string]string{
						generators.ArgoCDSecretTypeLabel: generators.ArgoCDSecretTypeCluster,
					},
				},
			},
			expectedRequests: []reconcile.Request{{
				NamespacedName: types.NamespacedName{Namespace: "argocd", Name: "my-app-set"},
			}},
		},
		{
			name: "multiple cluster generators should produce multiple requests",
			items: []argov1alpha1.ApplicationSet{
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-app-set",
						Namespace: "argocd",
					},
					Spec: argov1alpha1.ApplicationSetSpec{
						Generators: []argov1alpha1.ApplicationSetGenerator{
							{
								Clusters: &argov1alpha1.ClusterGenerator{},
							},
						},
					},
				},
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-app-set2",
						Namespace: "argocd",
					},
					Spec: argov1alpha1.ApplicationSetSpec{
						Generators: []argov1alpha1.ApplicationSetGenerator{
							{
								Clusters: &argov1alpha1.ClusterGenerator{},
							},
						},
					},
				},
			},
			secret: corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "argocd",
					Name:      "my-secret",
					Labels: map[string]string{
						generators.ArgoCDSecretTypeLabel: generators.ArgoCDSecretTypeCluster,
					},
				},
			},
			expectedRequests: []reconcile.Request{
				{NamespacedName: types.NamespacedName{Namespace: "argocd", Name: "my-app-set"}},
				{NamespacedName: types.NamespacedName{Namespace: "argocd", Name: "my-app-set2"}},
			},
		},
		{
			name: "non-cluster generator should not match",
			items: []argov1alpha1.ApplicationSet{
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-app-set",
						Namespace: "another-namespace",
					},
					Spec: argov1alpha1.ApplicationSetSpec{
						Generators: []argov1alpha1.ApplicationSetGenerator{
							{
								Clusters: &argov1alpha1.ClusterGenerator{},
							},
						},
					},
				},
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "app-set-non-cluster",
						Namespace: "argocd",
					},
					Spec: argov1alpha1.ApplicationSetSpec{
						Generators: []argov1alpha1.ApplicationSetGenerator{
							{
								List: &argov1alpha1.ListGenerator{},
							},
						},
					},
				},
			},
			secret: corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "argocd",
					Name:      "my-secret",
					Labels: map[string]string{
						generators.ArgoCDSecretTypeLabel: generators.ArgoCDSecretTypeCluster,
					},
				},
			},
			expectedRequests: []reconcile.Request{
				{NamespacedName: types.NamespacedName{Namespace: "another-namespace", Name: "my-app-set"}},
			},
		},

		{
			name: "non-argo cd secret should not match",
			items: []argov1alpha1.ApplicationSet{
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-app-set",
						Namespace: "another-namespace",
					},
					Spec: argov1alpha1.ApplicationSetSpec{
						Generators: []argov1alpha1.ApplicationSetGenerator{
							{
								Clusters: &argov1alpha1.ClusterGenerator{},
							},
						},
					},
				},
			},
			secret: corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "argocd",
					Name:      "my-non-argocd-secret",
				},
			},
			expectedRequests: []reconcile.Request{},
		},
	}

	for _, test := range tests {

		t.Run(test.name, func(t *testing.T) {

			appSetList := argov1alpha1.ApplicationSetList{
				Items: test.items,
			}

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithLists(&appSetList).Build()

			handler := &clusterSecretEventHandler{
				Client: fakeClient,
				Log:    log.WithField("type", "createSecretEventHandler"),
			}

			mockAddRateLimitingInterface := mockAddRateLimitingInterface{}

			handler.queueRelatedAppGenerators(&mockAddRateLimitingInterface, &test.secret)

			assert.False(t, mockAddRateLimitingInterface.errorOccurred)
			assert.ElementsMatch(t, mockAddRateLimitingInterface.addedItems, test.expectedRequests)

		})
	}

}

// Add checks the type, and adds it to the internal list of received additions
func (obj *mockAddRateLimitingInterface) Add(item interface{}) {
	if req, ok := item.(ctrl.Request); ok {
		obj.addedItems = append(obj.addedItems, req)
	} else {
		obj.errorOccurred = true
	}
}

type mockAddRateLimitingInterface struct {
	errorOccurred bool
	addedItems    []ctrl.Request
}
