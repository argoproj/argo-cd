package controllers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj/argo-cd/v3/applicationset/generators"
	appsetmetrics "github.com/argoproj/argo-cd/v3/applicationset/metrics"
	"github.com/argoproj/argo-cd/v3/applicationset/services/mocks"
	argov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestRequeueAfter(t *testing.T) {
	mockServer := &mocks.Repos{}
	ctx := t.Context()
	scheme := runtime.NewScheme()
	err := argov1alpha1.AddToScheme(scheme)
	require.NoError(t, err)
	gvrToListKind := map[schema.GroupVersionResource]string{{
		Group:    "mallard.io",
		Version:  "v1",
		Resource: "ducks",
	}: "DuckList"}
	appClientset := kubefake.NewSimpleClientset()
	k8sClient := fake.NewClientBuilder().Build()
	duckType := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v2quack",
			"kind":       "Duck",
			"metadata": map[string]any{
				"name":      "mightyduck",
				"namespace": "namespace",
				"labels":    map[string]any{"duck": "all-species"},
			},
			"status": map[string]any{
				"decisions": []any{
					map[string]any{
						"clusterName": "staging-01",
					},
					map[string]any{
						"clusterName": "production-01",
					},
				},
			},
		},
	}
	fakeDynClient := dynfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind, duckType)
	scmConfig := generators.NewSCMConfig("", []string{""}, true, true, nil, true)
	terminalGenerators := map[string]generators.Generator{
		"List":                    generators.NewListGenerator(),
		"Clusters":                generators.NewClusterGenerator(ctx, k8sClient, appClientset, "argocd"),
		"Git":                     generators.NewGitGenerator(mockServer, "namespace"),
		"SCMProvider":             generators.NewSCMProviderGenerator(fake.NewClientBuilder().WithObjects(&corev1.Secret{}).Build(), scmConfig),
		"ClusterDecisionResource": generators.NewDuckTypeGenerator(ctx, fakeDynClient, appClientset, "argocd"),
		"PullRequest":             generators.NewPullRequestGenerator(k8sClient, scmConfig),
	}

	nestedGenerators := map[string]generators.Generator{
		"List":                    terminalGenerators["List"],
		"Clusters":                terminalGenerators["Clusters"],
		"Git":                     terminalGenerators["Git"],
		"SCMProvider":             terminalGenerators["SCMProvider"],
		"ClusterDecisionResource": terminalGenerators["ClusterDecisionResource"],
		"PullRequest":             terminalGenerators["PullRequest"],
		"Matrix":                  generators.NewMatrixGenerator(terminalGenerators),
		"Merge":                   generators.NewMergeGenerator(terminalGenerators),
	}

	topLevelGenerators := map[string]generators.Generator{
		"List":                    terminalGenerators["List"],
		"Clusters":                terminalGenerators["Clusters"],
		"Git":                     terminalGenerators["Git"],
		"SCMProvider":             terminalGenerators["SCMProvider"],
		"ClusterDecisionResource": terminalGenerators["ClusterDecisionResource"],
		"PullRequest":             terminalGenerators["PullRequest"],
		"Matrix":                  generators.NewMatrixGenerator(nestedGenerators),
		"Merge":                   generators.NewMergeGenerator(nestedGenerators),
	}

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	metrics := appsetmetrics.NewFakeAppsetMetrics()
	r := ApplicationSetReconciler{
		Client:     client,
		Scheme:     scheme,
		Recorder:   record.NewFakeRecorder(0),
		Generators: topLevelGenerators,
		Metrics:    metrics,
	}

	type args struct {
		appset               *argov1alpha1.ApplicationSet
		requeueAfterOverride string
	}
	tests := []struct {
		name    string
		args    args
		want    time.Duration
		wantErr assert.ErrorAssertionFunc
	}{
		{name: "Cluster", args: args{
			appset: &argov1alpha1.ApplicationSet{
				Spec: argov1alpha1.ApplicationSetSpec{
					Generators: []argov1alpha1.ApplicationSetGenerator{{Clusters: &argov1alpha1.ClusterGenerator{}}},
				},
			}, requeueAfterOverride: "",
		}, want: generators.NoRequeueAfter, wantErr: assert.NoError},
		{name: "ClusterMergeNested", args: args{&argov1alpha1.ApplicationSet{
			Spec: argov1alpha1.ApplicationSetSpec{
				Generators: []argov1alpha1.ApplicationSetGenerator{
					{Clusters: &argov1alpha1.ClusterGenerator{}},
					{Merge: &argov1alpha1.MergeGenerator{
						Generators: []argov1alpha1.ApplicationSetNestedGenerator{
							{
								Clusters: &argov1alpha1.ClusterGenerator{},
								Git:      &argov1alpha1.GitGenerator{},
							},
						},
					}},
				},
			},
		}, ""}, want: generators.DefaultRequeueAfter, wantErr: assert.NoError},
		{name: "ClusterMatrixNested", args: args{&argov1alpha1.ApplicationSet{
			Spec: argov1alpha1.ApplicationSetSpec{
				Generators: []argov1alpha1.ApplicationSetGenerator{
					{Clusters: &argov1alpha1.ClusterGenerator{}},
					{Matrix: &argov1alpha1.MatrixGenerator{
						Generators: []argov1alpha1.ApplicationSetNestedGenerator{
							{
								Clusters: &argov1alpha1.ClusterGenerator{},
								Git:      &argov1alpha1.GitGenerator{},
							},
						},
					}},
				},
			},
		}, ""}, want: generators.DefaultRequeueAfter, wantErr: assert.NoError},
		{name: "ListGenerator", args: args{appset: &argov1alpha1.ApplicationSet{
			Spec: argov1alpha1.ApplicationSetSpec{
				Generators: []argov1alpha1.ApplicationSetGenerator{{List: &argov1alpha1.ListGenerator{}}},
			},
		}}, want: generators.NoRequeueAfter, wantErr: assert.NoError},
		{name: "DuckGenerator", args: args{appset: &argov1alpha1.ApplicationSet{
			Spec: argov1alpha1.ApplicationSetSpec{
				Generators: []argov1alpha1.ApplicationSetGenerator{{ClusterDecisionResource: &argov1alpha1.DuckTypeGenerator{}}},
			},
		}}, want: generators.DefaultRequeueAfter, wantErr: assert.NoError},
		{name: "OverrideRequeueDuck", args: args{
			appset: &argov1alpha1.ApplicationSet{
				Spec: argov1alpha1.ApplicationSetSpec{
					Generators: []argov1alpha1.ApplicationSetGenerator{{ClusterDecisionResource: &argov1alpha1.DuckTypeGenerator{}}},
				},
			}, requeueAfterOverride: "1h",
		}, want: 1 * time.Hour, wantErr: assert.NoError},
		{name: "OverrideRequeueGit", args: args{&argov1alpha1.ApplicationSet{
			Spec: argov1alpha1.ApplicationSetSpec{
				Generators: []argov1alpha1.ApplicationSetGenerator{
					{Git: &argov1alpha1.GitGenerator{}},
				},
			},
		}, "1h"}, want: 1 * time.Hour, wantErr: assert.NoError},
		{name: "OverrideRequeueMatrix", args: args{&argov1alpha1.ApplicationSet{
			Spec: argov1alpha1.ApplicationSetSpec{
				Generators: []argov1alpha1.ApplicationSetGenerator{
					{Clusters: &argov1alpha1.ClusterGenerator{}},
					{Merge: &argov1alpha1.MergeGenerator{
						Generators: []argov1alpha1.ApplicationSetNestedGenerator{
							{
								Clusters: &argov1alpha1.ClusterGenerator{},
								Git:      &argov1alpha1.GitGenerator{},
							},
						},
					}},
				},
			},
		}, "5m"}, want: 5 * time.Minute, wantErr: assert.NoError},
		{name: "OverrideRequeueMerge", args: args{&argov1alpha1.ApplicationSet{
			Spec: argov1alpha1.ApplicationSetSpec{
				Generators: []argov1alpha1.ApplicationSetGenerator{
					{Clusters: &argov1alpha1.ClusterGenerator{}},
					{Merge: &argov1alpha1.MergeGenerator{
						Generators: []argov1alpha1.ApplicationSetNestedGenerator{
							{
								Clusters: &argov1alpha1.ClusterGenerator{},
								Git:      &argov1alpha1.GitGenerator{},
							},
						},
					}},
				},
			},
		}, "12s"}, want: 12 * time.Second, wantErr: assert.NoError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ARGOCD_APPLICATIONSET_CONTROLLER_REQUEUE_AFTER", tt.args.requeueAfterOverride)
			assert.Equalf(t, tt.want, r.getMinRequeueAfter(tt.args.appset), "getMinRequeueAfter(%v)", tt.args.appset)
		})
	}
}
