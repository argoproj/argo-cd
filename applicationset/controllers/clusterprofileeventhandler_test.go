package controllers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	log "github.com/sirupsen/logrus"

	argov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	clusterinventory "sigs.k8s.io/cluster-inventory-api/apis/v1alpha1"
)

func (m *mockAddRateLimitingInterface) AddAfter(item interface{}, duration time.Duration) {
	// Not implemented
}

func (m *mockAddRateLimitingInterface) AddRateLimited(item interface{}) {
	// Not implemented
}

func (m *mockAddRateLimitingInterface) Done(item interface{}) {
	// Not implemented
}

func (m *mockAddRateLimitingInterface) Forget(item interface{}) {
	// Not implemented
}

func (m *mockAddRateLimitingInterface) Get() (item interface{}, shutdown bool) {
	return nil, false
}

func (m *mockAddRateLimitingInterface) Len() int {
	return 0
}

func (m *mockAddRateLimitingInterface) NumRequeues(item interface{}) int {
	return 0
}

func (m *mockAddRateLimitingInterface) ShutDown() {
	// Not implemented
}

func (m *mockAddRateLimitingInterface) ShuttingDown() bool {
	return false
}

func (m *mockAddRateLimitingInterface) Shutdown() {
	// Not implemented
}

func TestClusterProfileEventHandler(t *testing.T) {
	scheme := runtime.NewScheme()
	err := argov1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	tests := []struct {
		name             string
		items            []argov1alpha1.ApplicationSet
		profile          clusterinventory.ClusterProfile
		expectedRequests []ctrl.Request
	}{
		{
			name:  "no application sets should mean no requests",
			items: []argov1alpha1.ApplicationSet{},
			profile: clusterinventory.ClusterProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-profile",
				},
			},
			expectedRequests: []reconcile.Request{},
		},
		{
			name: "a cluster profile generator should produce a request",
			items: []argov1alpha1.ApplicationSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-app-set",
						Namespace: "argocd",
					},
					Spec: argov1alpha1.ApplicationSetSpec{
						Generators: []argov1alpha1.ApplicationSetGenerator{
							{
								ClusterProfiles: &argov1alpha1.ClusterProfileGenerator{},
							},
						},
					},
				},
			},
			profile: clusterinventory.ClusterProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-profile",
				},
			},
			expectedRequests: []reconcile.Request{{
				NamespacedName: types.NamespacedName{Namespace: "argocd", Name: "my-app-set"},
			}},
		},
		{
			name: "multiple cluster profile generators should produce multiple requests",
			items: []argov1alpha1.ApplicationSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-app-set",
						Namespace: "argocd",
					},
					Spec: argov1alpha1.ApplicationSetSpec{
						Generators: []argov1alpha1.ApplicationSetGenerator{
							{
								ClusterProfiles: &argov1alpha1.ClusterProfileGenerator{},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-app-set2",
						Namespace: "argocd",
					},
					Spec: argov1alpha1.ApplicationSetSpec{
						Generators: []argov1alpha1.ApplicationSetGenerator{
							{
								ClusterProfiles: &argov1alpha1.ClusterProfileGenerator{},
							},
						},
					},
				},
			},
			profile: clusterinventory.ClusterProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-profile",
				},
			},
			expectedRequests: []reconcile.Request{
				{NamespacedName: types.NamespacedName{Namespace: "argocd", Name: "my-app-set"}},
				{NamespacedName: types.NamespacedName{Namespace: "argocd", Name: "my-app-set2"}},
			},
		},
		{
			name: "non-cluster profile generator should not match",
			items: []argov1alpha1.ApplicationSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-app-set",
						Namespace: "another-namespace",
					},
					Spec: argov1alpha1.ApplicationSetSpec{
						Generators: []argov1alpha1.ApplicationSetGenerator{
							{
								ClusterProfiles: &argov1alpha1.ClusterProfileGenerator{},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-set-non-cluster-profile",
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
			profile: clusterinventory.ClusterProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-profile",
				},
			},
			expectedRequests: []reconcile.Request{
				{NamespacedName: types.NamespacedName{Namespace: "another-namespace", Name: "my-app-set"}},
			},
		},
		{
			name: "a matrix generator with a cluster profile generator should produce a request",
			items: []argov1alpha1.ApplicationSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-app-set",
						Namespace: "argocd",
					},
					Spec: argov1alpha1.ApplicationSetSpec{
						Generators: []argov1alpha1.ApplicationSetGenerator{
							{
								Matrix: &argov1alpha1.MatrixGenerator{
									Generators: []argov1alpha1.ApplicationSetNestedGenerator{
										{
											ClusterProfiles: &argov1alpha1.ClusterProfileGenerator{},
										},
									},
								},
							},
						},
					},
				},
			},
			profile: clusterinventory.ClusterProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-profile",
				},
			},
			expectedRequests: []reconcile.Request{{
				NamespacedName: types.NamespacedName{Namespace: "argocd", Name: "my-app-set"},
			}},
		},
		{
			name: "a matrix generator with non cluster profile generator should not match",
			items: []argov1alpha1.ApplicationSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-app-set",
						Namespace: "argocd",
					},
					Spec: argov1alpha1.ApplicationSetSpec{
						Generators: []argov1alpha1.ApplicationSetGenerator{
							{
								Matrix: &argov1alpha1.MatrixGenerator{
									Generators: []argov1alpha1.ApplicationSetNestedGenerator{
										{
											List: &argov1alpha1.ListGenerator{},
										},
									},
								},
							},
						},
					},
				},
			},
			profile: clusterinventory.ClusterProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-profile",
				},
			},
			expectedRequests: []reconcile.Request{},
		},
		{
			name: "a matrix generator with a nested matrix generator containing a cluster profile generator should produce a request",
			items: []argov1alpha1.ApplicationSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-app-set",
						Namespace: "argocd",
					},
					Spec: argov1alpha1.ApplicationSetSpec{
						Generators: []argov1alpha1.ApplicationSetGenerator{
							{
								Matrix: &argov1alpha1.MatrixGenerator{
									Generators: []argov1alpha1.ApplicationSetNestedGenerator{
										{
											Matrix: &apiextensionsv1.JSON{
												Raw: []byte(
													`{
														"generators": [
														  {
															"clusterProfiles": {
															  "selector": {
																"matchLabels": {
																  "argocd.argoproj.io/secret-type": "cluster"
																}
															  }
															}
														  }
														]
													  }`,
												),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			profile: clusterinventory.ClusterProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-profile",
				},
			},
			expectedRequests: []reconcile.Request{{
				NamespacedName: types.NamespacedName{Namespace: "argocd", Name: "my-app-set"},
			}},
		},
		{
			name: "a matrix generator with a nested matrix generator containing non cluster profile generator should not match",
			items: []argov1alpha1.ApplicationSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-app-set",
						Namespace: "argocd",
					},
					Spec: argov1alpha1.ApplicationSetSpec{
						Generators: []argov1alpha1.ApplicationSetGenerator{
							{
								Matrix: &argov1alpha1.MatrixGenerator{
									Generators: []argov1alpha1.ApplicationSetNestedGenerator{
										{
											Matrix: &apiextensionsv1.JSON{
												Raw: []byte(
													`{
														"generators": [
														  {
															"list": {
															  "elements": [
																"a",
																"b"
															  ]
															}
														  }
														]
													  }`),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			profile: clusterinventory.ClusterProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-profile",
				},
			},
			expectedRequests: []reconcile.Request{},
		},
		{
			name: "a merge generator with a cluster profile generator should produce a request",
			items: []argov1alpha1.ApplicationSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-app-set",
						Namespace: "argocd",
					},
					Spec: argov1alpha1.ApplicationSetSpec{
						Generators: []argov1alpha1.ApplicationSetGenerator{
							{
								Merge: &argov1alpha1.MergeGenerator{
									Generators: []argov1alpha1.ApplicationSetNestedGenerator{
										{
											ClusterProfiles: &argov1alpha1.ClusterProfileGenerator{},
										},
									},
								},
							},
						},
					},
				},
			},
			profile: clusterinventory.ClusterProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-profile",
				},
			},
			expectedRequests: []reconcile.Request{{
				NamespacedName: types.NamespacedName{Namespace: "argocd", Name: "my-app-set"},
			}},
		},
		{
			name: "a merge generator with non cluster profile generator should not match",
			items: []argov1alpha1.ApplicationSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-app-set",
						Namespace: "argocd",
					},
					Spec: argov1alpha1.ApplicationSetSpec{
						Generators: []argov1alpha1.ApplicationSetGenerator{
							{
								Merge: &argov1alpha1.MergeGenerator{
									Generators: []argov1alpha1.ApplicationSetNestedGenerator{
										{
											List: &argov1alpha1.ListGenerator{},
										},
									},
								},
							},
						},
					},
				},
			},
			profile: clusterinventory.ClusterProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-profile",
				},
			},
			expectedRequests: []reconcile.Request{},
		},
		{
			name: "a merge generator with a nested merge generator containing a cluster profile generator should produce a request",
			items: []argov1alpha1.ApplicationSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-app-set",
						Namespace: "argocd",
					},
					Spec: argov1alpha1.ApplicationSetSpec{
						Generators: []argov1alpha1.ApplicationSetGenerator{
							{
								Merge: &argov1alpha1.MergeGenerator{
									Generators: []argov1alpha1.ApplicationSetNestedGenerator{
										{
											Merge: &apiextensionsv1.JSON{
												Raw: []byte(
													`{
														"generators": [
														  {
															"clusterProfiles": {
															  "selector": {
																"matchLabels": {
																  "foo": "bar"
																}
															  }
															}
														  }
														]
													  }`,
												),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			profile: clusterinventory.ClusterProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-profile",
				},
			},
			expectedRequests: []reconcile.Request{{
				NamespacedName: types.NamespacedName{Namespace: "argocd", Name: "my-app-set"},
			}},
		},
		{
			name: "a merge generator with a nested merge generator containing non cluster profile generator should not match",
			items: []argov1alpha1.ApplicationSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-app-set",
						Namespace: "argocd",
					},
					Spec: argov1alpha1.ApplicationSetSpec{
						Generators: []argov1alpha1.ApplicationSetGenerator{
							{
								Merge: &argov1alpha1.MergeGenerator{
									Generators: []argov1alpha1.ApplicationSetNestedGenerator{
										{
											Merge: &apiextensionsv1.JSON{
												Raw: []byte(
													`{
														"generators": [
														  {
															"list": {
															  "elements": [
																"a",
																"b"
															  ]
															}
														  }
														]
													  }`,
												),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			profile: clusterinventory.ClusterProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-profile",
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

			handler := &clusterProfileEventHandler{
				Client: fakeClient,
				Log:    log.WithField("type", "createClusterProfileEventHandler"),
			}

			mockAddRateLimitingInterface := mockAddRateLimitingInterface{}

			handler.queueRelatedAppGenerators(t.Context(), &mockAddRateLimitingInterface, &test.profile)

			assert.ElementsMatch(t, mockAddRateLimitingInterface.addedItems, test.expectedRequests)
		})
	}
}

func TestNestedGeneratorHasClusterProfileGenerator_NestedClusterProfileGenerator(t *testing.T) {
	nested := argov1alpha1.ApplicationSetNestedGenerator{
		ClusterProfiles: &argov1alpha1.ClusterProfileGenerator{},
	}

	hasClusterProfileGenerator, err := nestedGeneratorHasClusterProfileGenerator(nested)

	require.NoError(t, err)
	assert.True(t, hasClusterProfileGenerator)
}

func TestNestedGeneratorHasClusterProfileGenerator_NestedMergeGenerator(t *testing.T) {
	nested := argov1alpha1.ApplicationSetNestedGenerator{
		Merge: &apiextensionsv1.JSON{
			Raw: []byte(
				`{
					"generators": [
					  {
						"clusterProfiles": {
						  "selector": {
							"matchLabels": {
							  "foo": "bar"
							}
						  }
						}
					  }
					]
				  }`,
			),
		},
	}

	hasClusterProfileGenerator, err := nestedGeneratorHasClusterProfileGenerator(nested)

	require.NoError(t, err)
	assert.True(t, hasClusterProfileGenerator)
}

func TestNestedGeneratorHasClusterProfileGenerator_NestedMergeGeneratorWithInvalidJSON(t *testing.T) {
	nested := argov1alpha1.ApplicationSetNestedGenerator{
		Merge: &apiextensionsv1.JSON{
			Raw: []byte(
				`{
					"generators": [
					  {
						"clusterProfiles": {
						  "selector": {
							"matchLabels": {
							  "foo": "bar"
							}
						  }
						}
					  }
					]
				  `,
			),
		},
	}

	hasClusterProfileGenerator, err := nestedGeneratorHasClusterProfileGenerator(nested)

	require.Error(t, err)
	assert.False(t, hasClusterProfileGenerator)
}
