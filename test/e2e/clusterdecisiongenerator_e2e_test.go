package e2e

import (
	"testing"

	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/applicationsets"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture/applicationsets/utils"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application"
)

var tenSec = int64(10)

func TestSimpleClusterDecisionResourceGeneratorExternalNamespace(t *testing.T) {
	externalNamespace := string(utils.ArgoCDExternalNamespace)

	expectedApp := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "cluster1-guestbook",
			Namespace:  externalNamespace,
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: &argov1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				TargetRevision: "HEAD",
				Path:           "guestbook",
			},
			Destination: argov1alpha1.ApplicationDestination{
				Name:      "cluster1",
				Namespace: "guestbook",
			},
		},
	}

	var expectedAppNewNamespace *argov1alpha1.Application
	var expectedAppNewMetadata *argov1alpha1.Application

	clusterList := []interface{}{
		map[string]interface{}{
			"clusterName": "cluster1",
			"reason":      "argotest",
		},
	}

	Given(t).
		// Create a ClusterGenerator-based ApplicationSet
		When().
		CreateClusterSecret("my-secret", "cluster1", "https://kubernetes.default.svc").
		CreatePlacementRoleAndRoleBinding().
		CreatePlacementDecisionConfigMap("my-configmap").
		CreatePlacementDecision("my-placementdecision").
		StatusUpdatePlacementDecision("my-placementdecision", clusterList).
		CreateNamespace(externalNamespace).
		SwitchToExternalNamespace(utils.ArgoCDExternalNamespace).
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-cluster-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{name}}-guestbook"},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: &argov1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "guestbook",
						},
						Destination: argov1alpha1.ApplicationDestination{
							Name: "{{clusterName}}",
							// Server:    "{{server}}",
							Namespace: "guestbook",
						},
					},
				},
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						ClusterDecisionResource: &v1alpha1.DuckTypeGenerator{
							ConfigMapRef: "my-configmap",
							Name:         "my-placementdecision",
						},
					},
				},
			},
		}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{expectedApp})).

		// Update the ApplicationSet template namespace, and verify it updates the Applications
		When().
		And(func() {
			expectedAppNewNamespace = expectedApp.DeepCopy()
			expectedAppNewNamespace.Spec.Destination.Namespace = "guestbook2"
		}).
		Update(func(appset *v1alpha1.ApplicationSet) {
			appset.Spec.Template.Spec.Destination.Namespace = "guestbook2"
		}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{*expectedAppNewNamespace})).

		// Update the metadata fields in the appset template, and make sure it propagates to the apps
		When().
		And(func() {
			expectedAppNewMetadata = expectedAppNewNamespace.DeepCopy()
			expectedAppNewMetadata.ObjectMeta.Annotations = map[string]string{"annotation-key": "annotation-value"}
			expectedAppNewMetadata.ObjectMeta.Labels = map[string]string{
				"label-key": "label-value",
			}
		}).
		Update(func(appset *v1alpha1.ApplicationSet) {
			appset.Spec.Template.Annotations = map[string]string{"annotation-key": "annotation-value"}
			appset.Spec.Template.Labels = map[string]string{
				"label-key": "label-value",
			}
		}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{*expectedAppNewMetadata})).

		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist([]argov1alpha1.Application{*expectedAppNewNamespace}))
}

func TestSimpleClusterDecisionResourceGenerator(t *testing.T) {
	expectedApp := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "cluster1-guestbook",
			Namespace:  fixture.TestNamespace(),
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: &argov1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				TargetRevision: "HEAD",
				Path:           "guestbook",
			},
			Destination: argov1alpha1.ApplicationDestination{
				Name:      "cluster1",
				Namespace: "guestbook",
			},
		},
	}

	var expectedAppNewNamespace *argov1alpha1.Application
	var expectedAppNewMetadata *argov1alpha1.Application

	clusterList := []interface{}{
		map[string]interface{}{
			"clusterName": "cluster1",
			"reason":      "argotest",
		},
	}

	Given(t).
		// Create a ClusterGenerator-based ApplicationSet
		When().
		CreateClusterSecret("my-secret", "cluster1", "https://kubernetes.default.svc").
		CreatePlacementRoleAndRoleBinding().
		CreatePlacementDecisionConfigMap("my-configmap").
		CreatePlacementDecision("my-placementdecision").
		StatusUpdatePlacementDecision("my-placementdecision", clusterList).
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-cluster-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{name}}-guestbook"},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: &argov1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "guestbook",
						},
						Destination: argov1alpha1.ApplicationDestination{
							Name: "{{clusterName}}",
							// Server:    "{{server}}",
							Namespace: "guestbook",
						},
					},
				},
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						ClusterDecisionResource: &v1alpha1.DuckTypeGenerator{
							ConfigMapRef: "my-configmap",
							Name:         "my-placementdecision",
						},
					},
				},
			},
		}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{expectedApp})).

		// Update the ApplicationSet template namespace, and verify it updates the Applications
		When().
		And(func() {
			expectedAppNewNamespace = expectedApp.DeepCopy()
			expectedAppNewNamespace.Spec.Destination.Namespace = "guestbook2"
		}).
		Update(func(appset *v1alpha1.ApplicationSet) {
			appset.Spec.Template.Spec.Destination.Namespace = "guestbook2"
		}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{*expectedAppNewNamespace})).

		// Update the metadata fields in the appset template, and make sure it propagates to the apps
		When().
		And(func() {
			expectedAppNewMetadata = expectedAppNewNamespace.DeepCopy()
			expectedAppNewMetadata.ObjectMeta.Annotations = map[string]string{"annotation-key": "annotation-value"}
			expectedAppNewMetadata.ObjectMeta.Labels = map[string]string{"label-key": "label-value"}
		}).
		Update(func(appset *v1alpha1.ApplicationSet) {
			appset.Spec.Template.Annotations = map[string]string{"annotation-key": "annotation-value"}
			appset.Spec.Template.Labels = map[string]string{"label-key": "label-value"}
		}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{*expectedAppNewMetadata})).

		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist([]argov1alpha1.Application{*expectedAppNewNamespace}))
}

func TestSimpleClusterDecisionResourceGeneratorAddingCluster(t *testing.T) {
	expectedAppTemplate := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "{{name}}-guestbook",
			Namespace:  fixture.TestNamespace(),
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: &argov1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				TargetRevision: "HEAD",
				Path:           "guestbook",
			},
			Destination: argov1alpha1.ApplicationDestination{
				Name:      "{{clusterName}}",
				Namespace: "guestbook",
			},
		},
	}

	expectedAppCluster1 := *expectedAppTemplate.DeepCopy()
	expectedAppCluster1.Spec.Destination.Name = "cluster1"
	expectedAppCluster1.ObjectMeta.Name = "cluster1-guestbook"

	expectedAppCluster2 := *expectedAppTemplate.DeepCopy()
	expectedAppCluster2.Spec.Destination.Name = "cluster2"
	expectedAppCluster2.ObjectMeta.Name = "cluster2-guestbook"

	clusterList := []interface{}{
		map[string]interface{}{
			"clusterName": "cluster1",
			"reason":      "argotest",
		},
		map[string]interface{}{
			"clusterName": "cluster2",
			"reason":      "argotest",
		},
	}

	Given(t).
		// Create a ClusterGenerator-based ApplicationSet
		When().
		CreateClusterSecret("my-secret", "cluster1", "https://kubernetes.default.svc").
		CreatePlacementRoleAndRoleBinding().
		CreatePlacementDecisionConfigMap("my-configmap").
		CreatePlacementDecision("my-placementdecision").
		StatusUpdatePlacementDecision("my-placementdecision", clusterList).
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-cluster-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{name}}-guestbook"},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: &argov1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "guestbook",
						},
						Destination: argov1alpha1.ApplicationDestination{
							Name: "{{clusterName}}",
							// Server:    "{{server}}",
							Namespace: "guestbook",
						},
					},
				},
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						ClusterDecisionResource: &v1alpha1.DuckTypeGenerator{
							ConfigMapRef:        "my-configmap",
							Name:                "my-placementdecision",
							RequeueAfterSeconds: &tenSec,
						},
					},
				},
			},
		}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{expectedAppCluster1})).

		// Update the ApplicationSet template namespace, and verify it updates the Applications
		When().
		CreateClusterSecret("my-secret2", "cluster2", "https://kubernetes.default.svc").
		Then().Expect(ApplicationsExist([]argov1alpha1.Application{expectedAppCluster1, expectedAppCluster2})).

		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist([]argov1alpha1.Application{expectedAppCluster1, expectedAppCluster2}))
}

func TestSimpleClusterDecisionResourceGeneratorDeletingClusterSecret(t *testing.T) {
	expectedAppTemplate := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "{{name}}-guestbook",
			Namespace:  fixture.TestNamespace(),
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: &argov1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				TargetRevision: "HEAD",
				Path:           "guestbook",
			},
			Destination: argov1alpha1.ApplicationDestination{
				Name:      "{{name}}",
				Namespace: "guestbook",
			},
		},
	}

	expectedAppCluster1 := *expectedAppTemplate.DeepCopy()
	expectedAppCluster1.Spec.Destination.Name = "cluster1"
	expectedAppCluster1.ObjectMeta.Name = "cluster1-guestbook"

	expectedAppCluster2 := *expectedAppTemplate.DeepCopy()
	expectedAppCluster2.Spec.Destination.Name = "cluster2"
	expectedAppCluster2.ObjectMeta.Name = "cluster2-guestbook"

	clusterList := []interface{}{
		map[string]interface{}{
			"clusterName": "cluster1",
			"reason":      "argotest",
		},
		map[string]interface{}{
			"clusterName": "cluster2",
			"reason":      "argotest",
		},
	}

	Given(t).
		// Create a ClusterGenerator-based ApplicationSet
		When().
		CreateClusterSecret("my-secret", "cluster1", "https://kubernetes.default.svc").
		CreateClusterSecret("my-secret2", "cluster2", "https://kubernetes.default.svc").
		CreatePlacementRoleAndRoleBinding().
		CreatePlacementDecisionConfigMap("my-configmap").
		CreatePlacementDecision("my-placementdecision").
		StatusUpdatePlacementDecision("my-placementdecision", clusterList).
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-cluster-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{name}}-guestbook"},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: &argov1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "guestbook",
						},
						Destination: argov1alpha1.ApplicationDestination{
							Name: "{{clusterName}}",
							// Server:    "{{server}}",
							Namespace: "guestbook",
						},
					},
				},
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						ClusterDecisionResource: &v1alpha1.DuckTypeGenerator{
							ConfigMapRef:        "my-configmap",
							Name:                "my-placementdecision",
							RequeueAfterSeconds: &tenSec,
						},
					},
				},
			},
		}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{expectedAppCluster1, expectedAppCluster2})).

		// Update the ApplicationSet template namespace, and verify it updates the Applications
		When().
		DeleteClusterSecret("my-secret2").
		Then().Expect(ApplicationsExist([]argov1alpha1.Application{expectedAppCluster1})).
		Expect(ApplicationsDoNotExist([]argov1alpha1.Application{expectedAppCluster2})).

		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist([]argov1alpha1.Application{expectedAppCluster1}))
}

func TestSimpleClusterDecisionResourceGeneratorDeletingClusterFromResource(t *testing.T) {
	expectedAppTemplate := argov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "{{name}}-guestbook",
			Namespace:  fixture.TestNamespace(),
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: &argov1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				TargetRevision: "HEAD",
				Path:           "guestbook",
			},
			Destination: argov1alpha1.ApplicationDestination{
				Name:      "{{name}}",
				Namespace: "guestbook",
			},
		},
	}

	expectedAppCluster1 := *expectedAppTemplate.DeepCopy()
	expectedAppCluster1.Spec.Destination.Name = "cluster1"
	expectedAppCluster1.ObjectMeta.Name = "cluster1-guestbook"

	expectedAppCluster2 := *expectedAppTemplate.DeepCopy()
	expectedAppCluster2.Spec.Destination.Name = "cluster2"
	expectedAppCluster2.ObjectMeta.Name = "cluster2-guestbook"

	clusterList := []interface{}{
		map[string]interface{}{
			"clusterName": "cluster1",
			"reason":      "argotest",
		},
		map[string]interface{}{
			"clusterName": "cluster2",
			"reason":      "argotest",
		},
	}

	clusterListSmall := []interface{}{
		map[string]interface{}{
			"clusterName": "cluster1",
			"reason":      "argotest",
		},
	}

	Given(t).
		// Create a ClusterGenerator-based ApplicationSet
		When().
		CreateClusterSecret("my-secret", "cluster1", "https://kubernetes.default.svc").
		CreateClusterSecret("my-secret2", "cluster2", "https://kubernetes.default.svc").
		CreatePlacementRoleAndRoleBinding().
		CreatePlacementDecisionConfigMap("my-configmap").
		CreatePlacementDecision("my-placementdecision").
		StatusUpdatePlacementDecision("my-placementdecision", clusterList).
		Create(v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simple-cluster-generator",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{name}}-guestbook"},
					Spec: argov1alpha1.ApplicationSpec{
						Project: "default",
						Source: &argov1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "guestbook",
						},
						Destination: argov1alpha1.ApplicationDestination{
							Name: "{{clusterName}}",
							// Server:    "{{server}}",
							Namespace: "guestbook",
						},
					},
				},
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						ClusterDecisionResource: &v1alpha1.DuckTypeGenerator{
							ConfigMapRef:        "my-configmap",
							Name:                "my-placementdecision",
							RequeueAfterSeconds: &tenSec,
						},
					},
				},
			},
		}).Then().Expect(ApplicationsExist([]argov1alpha1.Application{expectedAppCluster1, expectedAppCluster2})).

		// Update the ApplicationSet template namespace, and verify it updates the Applications
		When().
		StatusUpdatePlacementDecision("my-placementdecision", clusterListSmall).
		Then().Expect(ApplicationsExist([]argov1alpha1.Application{expectedAppCluster1})).
		Expect(ApplicationsDoNotExist([]argov1alpha1.Application{expectedAppCluster2})).

		// Delete the ApplicationSet, and verify it deletes the Applications
		When().
		Delete().Then().Expect(ApplicationsDoNotExist([]argov1alpha1.Application{expectedAppCluster1}))
}
