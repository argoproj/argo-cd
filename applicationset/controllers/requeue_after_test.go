package controllers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	"github.com/argoproj/argo-cd/v3/util/settings"
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
	clusterInformer, err := settings.NewClusterInformer(appClientset, "argocd")
	require.NoError(t, err)

	defer startAndSyncInformer(t, clusterInformer)()

	terminalGenerators := map[string]generators.Generator{
		"List":                    generators.NewListGenerator(),
		"Clusters":                generators.NewClusterGenerator(k8sClient, "argocd"),
		"Git":                     generators.NewGitGenerator(mockServer, "namespace"),
		"SCMProvider":             generators.NewSCMProviderGenerator(fake.NewClientBuilder().WithObjects(&corev1.Secret{}).Build(), scmConfig),
		"ClusterDecisionResource": generators.NewDuckTypeGenerator(ctx, fakeDynClient, appClientset, "argocd", clusterInformer),
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

// TestGetMinRequeueAfterDuringGracePeriod covers the case where an ApplicationSet only
// uses watch-driven generators (List/Clusters), which report generators.NoRequeueAfter (0)
// With gracePeriod awareness, getMinRequeueAfter should return a value depending on time elapsed wrt grace-period
func TestGetMinRequeueAfterDuringGracePeriod(t *testing.T) {
	scheme := runtime.NewScheme()
	err := argov1alpha1.AddToScheme(scheme)
	require.NoError(t, err)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	r := ApplicationSetReconciler{
		Client:     client,
		Scheme:     scheme,
		Recorder:   record.NewFakeRecorder(0),
		Generators: map[string]generators.Generator{"List": generators.NewListGenerator()},
		Metrics:    appsetmetrics.NewFakeAppsetMetrics(),
	}

	newListGeneratorAppSet := func(lastTransitionTime metav1.Time) *argov1alpha1.ApplicationSet {
		return &argov1alpha1.ApplicationSet{
			Spec: argov1alpha1.ApplicationSetSpec{
				Generators: []argov1alpha1.ApplicationSetGenerator{{List: &argov1alpha1.ListGenerator{}}},
				Strategy: &argov1alpha1.ApplicationSetStrategy{
					Type: "RollingSync",
					RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{
						Steps: []argov1alpha1.ApplicationSetRolloutStep{{}},
					},
				},
			},
			Status: argov1alpha1.ApplicationSetStatus{
				ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{
					{
						Application:        "app1",
						Status:             argov1alpha1.ProgressiveSyncWaiting,
						Message:            "Application has pending changes, setting status to Waiting",
						LastTransitionTime: &lastTransitionTime,
					},
				},
			},
		}
	}

	tests := []struct {
		name                      string
		appset                    *argov1alpha1.ApplicationSet
		enableProgressiveSyncs    bool
		refreshGracePeriodSeconds int //default is 30s, can explicitly set to be different
		want                      time.Duration
		wantDelta                 float64
	}{
		{
			name: "grace period still active, expect requeueAfter to be after remaining time",
			// 10s into a 30s grace period -> ~20s remaining.
			appset:                    newListGeneratorAppSet(metav1.NewTime(time.Now().Add(-10 * time.Second))),
			enableProgressiveSyncs:    true,
			refreshGracePeriodSeconds: 30,
			want:                      20 * time.Second,
			wantDelta:                 1,
		},
		{
			name:                      "grace period already elapsed, expect requeueAfter to be immediate",
			appset:                    newListGeneratorAppSet(metav1.NewTime(time.Now().Add(-60 * time.Second))),
			enableProgressiveSyncs:    true,
			refreshGracePeriodSeconds: 30,
			want:                      time.Second,
		},
		{
			name:                      "progressive syncs disabled, ignores grace-period",
			appset:                    newListGeneratorAppSet(metav1.NewTime(time.Now().Add(-10 * time.Second))),
			enableProgressiveSyncs:    false,
			refreshGracePeriodSeconds: 30,
			// grace-period awareness only kicks in when progressive syncs are enabled.
			want: generators.NoRequeueAfter,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r.EnableProgressiveSyncs = tt.enableProgressiveSyncs
			r.RefreshGracePeriodSeconds = tt.refreshGracePeriodSeconds
			if tt.wantDelta > 0 {
				assert.InEpsilon(t, tt.want, r.getMinRequeueAfter(tt.appset), tt.wantDelta)
			} else {
				assert.Equal(t, tt.want, r.getMinRequeueAfter(tt.appset))
			}
		})
	}
}
