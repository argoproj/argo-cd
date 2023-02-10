package controllers

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v2/applicationset/generators"
	argov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

func TestRequeueAfter(t *testing.T) {
	mockServer := argoCDServiceMock{}
	ctx := context.Background()
	scheme := runtime.NewScheme()
	err := argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)
	err = argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)
	gvrToListKind := map[schema.GroupVersionResource]string{{
		Group:    "mallard.io",
		Version:  "v1",
		Resource: "ducks",
	}: "DuckList"}
	appClientset := kubefake.NewSimpleClientset()
	k8sClient := fake.NewClientBuilder().Build()
	duckType := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v2quack",
			"kind":       "Duck",
			"metadata": map[string]interface{}{
				"name":      "mightyduck",
				"namespace": "namespace",
				"labels":    map[string]interface{}{"duck": "all-species"},
			},
			"status": map[string]interface{}{
				"decisions": []interface{}{
					map[string]interface{}{
						"clusterName": "staging-01",
					},
					map[string]interface{}{
						"clusterName": "production-01",
					},
				},
			},
		},
	}
	fakeDynClient := dynfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind, duckType)

	terminalGenerators := map[string]generators.Generator{
		"List":                    generators.NewListGenerator(),
		"Clusters":                generators.NewClusterGenerator(k8sClient, ctx, appClientset, "argocd"),
		"Git":                     generators.NewGitGenerator(mockServer),
		"SCMProvider":             generators.NewSCMProviderGenerator(fake.NewClientBuilder().WithObjects(&corev1.Secret{}).Build(), generators.SCMAuthProviders{}),
		"ClusterDecisionResource": generators.NewDuckTypeGenerator(ctx, fakeDynClient, appClientset, "argocd"),
		"PullRequest":             generators.NewPullRequestGenerator(k8sClient, generators.SCMAuthProviders{}),
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
	r := ApplicationSetReconciler{
		Client:     client,
		Scheme:     scheme,
		Recorder:   record.NewFakeRecorder(0),
		Generators: topLevelGenerators,
	}

	brokenCluster := getAppsetFromFile("testData/broken-cluster-appset.yaml", t)
	brokenNested := getAppsetFromFile("testData/broken-nested-appset.yaml", t)
	worksNested := getAppsetFromFile("testData/works-nested-appset.yaml", t)

	type args struct {
		appset *argov1alpha1.ApplicationSet
	}
	tests := []struct {
		name    string
		args    args
		want    time.Duration
		wantErr assert.ErrorAssertionFunc
	}{
		{name: "BrokenCluster", args: args{appset: brokenCluster}, want: generators.DefaultRequeueAfterSeconds, wantErr: assert.NoError},
		{name: "BrokenNested", args: args{appset: brokenNested}, want: generators.DefaultRequeueAfterSeconds, wantErr: assert.NoError},
		{name: "WorksNested", args: args{appset: worksNested}, want: generators.DefaultRequeueAfterSeconds, wantErr: assert.NoError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, r.getMinRequeueAfter(tt.args.appset), "getMinRequeueAfter(%v)", tt.args.appset)
		})
	}
}

func getAppsetFromFile(path string, t *testing.T) *argov1alpha1.ApplicationSet {
	file, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var appset argov1alpha1.ApplicationSet
	err = yaml.Unmarshal(file, &appset)
	if err != nil {
		t.Fatal(err)
	}
	return &appset
}

type argoCDServiceMock struct {
	mock *mock.Mock
}

func (a argoCDServiceMock) GetApps(ctx context.Context, repoURL string, revision string) ([]string, error) {
	args := a.mock.Called(ctx, repoURL, revision)

	return args.Get(0).([]string), args.Error(1)
}

func (a argoCDServiceMock) GetFiles(ctx context.Context, repoURL string, revision string, pattern string) (map[string][]byte, error) {
	args := a.mock.Called(ctx, repoURL, revision, pattern)

	return args.Get(0).(map[string][]byte), args.Error(1)
}

func (a argoCDServiceMock) GetFileContent(ctx context.Context, repoURL string, revision string, path string) ([]byte, error) {
	args := a.mock.Called(ctx, repoURL, revision, path)

	return args.Get(0).([]byte), args.Error(1)
}

func (a argoCDServiceMock) GetDirectories(ctx context.Context, repoURL string, revision string) ([]string, error) {
	args := a.mock.Called(ctx, repoURL, revision)
	return args.Get(0).([]string), args.Error(1)
}
