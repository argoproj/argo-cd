package utils

import (
	"context"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
)

func Test_applyIgnoreDifferences(t *testing.T) {
	t.Parallel()

	appMeta := metav1.TypeMeta{
		APIVersion: v1alpha1.ApplicationSchemaGroupVersionKind.GroupVersion().String(),
		Kind:       v1alpha1.ApplicationSchemaGroupVersionKind.Kind,
	}
	testCases := []struct {
		name              string
		ignoreDifferences v1alpha1.ApplicationSetIgnoreDifferences
		foundApp          string
		generatedApp      string
		expectedApp       string
	}{
		{
			name: "empty ignoreDifferences",
			foundApp: `
spec: {}`,
			generatedApp: `
spec: {}`,
			expectedApp: `
spec: {}`,
		},
		{
			// For this use case: https://github.com/argoproj/argo-cd/issues/9101#issuecomment-1191138278
			name: "ignore target revision with jq",
			ignoreDifferences: v1alpha1.ApplicationSetIgnoreDifferences{
				{JQPathExpressions: []string{".spec.source.targetRevision"}},
			},
			foundApp: `
spec:
  source:
    targetRevision: foo`,
			generatedApp: `
spec:
  source:
    targetRevision: bar`,
			expectedApp: `
spec:
  source:
    targetRevision: foo`,
		},
		{
			// For this use case: https://github.com/argoproj/argo-cd/issues/9101#issuecomment-1103593714
			name: "ignore helm parameter with jq",
			ignoreDifferences: v1alpha1.ApplicationSetIgnoreDifferences{
				{JQPathExpressions: []string{`.spec.source.helm.parameters | select(.name == "image.tag")`}},
			},
			foundApp: `
spec:
  source:
    helm:
      parameters:
      - name: image.tag
        value: test
      - name: another
        value: value`,
			generatedApp: `
spec:
  source:
    helm:
      parameters:
      - name: image.tag
        value: v1.0.0
      - name: another
        value: value`,
			expectedApp: `
spec:
  source:
    helm:
      parameters:
      - name: image.tag
        value: test
      - name: another
        value: value`,
		},
		{
			// For this use case: https://github.com/argoproj/argo-cd/issues/9101#issuecomment-1191138278
			name: "ignore auto-sync in appset when it's not in the cluster with jq",
			ignoreDifferences: v1alpha1.ApplicationSetIgnoreDifferences{
				{JQPathExpressions: []string{".spec.syncPolicy.automated"}},
			},
			foundApp: `
spec:
  syncPolicy:
    retry:
      limit: 5`,
			generatedApp: `
spec:
  syncPolicy:
    automated:
      selfHeal: true
    retry:
      limit: 5`,
			expectedApp: `
spec:
  syncPolicy:
    retry:
      limit: 5`,
		},
		{
			name: "ignore auto-sync in the cluster when it's not in the appset with jq",
			ignoreDifferences: v1alpha1.ApplicationSetIgnoreDifferences{
				{JQPathExpressions: []string{".spec.syncPolicy.automated"}},
			},
			foundApp: `
spec:
  syncPolicy:
    automated:
      selfHeal: true
    retry:
      limit: 5`,
			generatedApp: `
spec:
  syncPolicy:
    retry:
      limit: 5`,
			expectedApp: `
spec:
  syncPolicy:
    automated:
      selfHeal: true
    retry:
      limit: 5`,
		},
		{
			// For this use case: https://github.com/argoproj/argo-cd/issues/9101#issuecomment-1420656537
			name: "ignore a one-off annotation with jq",
			ignoreDifferences: v1alpha1.ApplicationSetIgnoreDifferences{
				{JQPathExpressions: []string{`.metadata.annotations | select(.["foo.bar"] == "baz")`}},
			},
			foundApp: `
metadata:
  annotations:
    foo.bar: baz
    some.other: annotation`,
			generatedApp: `
metadata:
  annotations:
    some.other: annotation`,
			expectedApp: `
metadata:
  annotations:
    foo.bar: baz
    some.other: annotation`,
		},
		{
			// For this use case: https://github.com/argoproj/argo-cd/issues/9101#issuecomment-1515672638
			name: "ignore the source.plugin field with a json pointer",
			ignoreDifferences: v1alpha1.ApplicationSetIgnoreDifferences{
				{JSONPointers: []string{"/spec/source/plugin"}},
			},
			foundApp: `
spec:
  source:
    plugin:
      parameters:
      - name: url
        string: https://example.com`,
			generatedApp: `
spec:
  source:
    plugin:
      parameters:
      - name: url
        string: https://example.com/wrong`,
			expectedApp: `
spec:
  source:
    plugin:
      parameters:
      - name: url
        string: https://example.com`,
		},
		{
			// For this use case: https://github.com/argoproj/argo-cd/pull/14743#issuecomment-1761954799
			name: "ignore parameters added to a multi-source app in the cluster",
			ignoreDifferences: v1alpha1.ApplicationSetIgnoreDifferences{
				{JQPathExpressions: []string{`.spec.sources[] | select(.repoURL | contains("test-repo")).helm.parameters`}},
			},
			foundApp: `
spec:
  sources:
  - repoURL: https://git.example.com/test-org/test-repo
    helm:
      parameters:
      - name: test
        value: hi`,
			generatedApp: `
spec:
  sources:
  - repoURL: https://git.example.com/test-org/test-repo`,
			expectedApp: `
spec:
  sources:
  - repoURL: https://git.example.com/test-org/test-repo
    helm:
      parameters:
      - name: test
        value: hi`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			foundApp := v1alpha1.Application{TypeMeta: appMeta}
			err := yaml.Unmarshal([]byte(tc.foundApp), &foundApp)
			require.NoError(t, err, tc.foundApp)
			generatedApp := v1alpha1.Application{TypeMeta: appMeta}
			err = yaml.Unmarshal([]byte(tc.generatedApp), &generatedApp)
			require.NoError(t, err, tc.generatedApp)
			diffConfig, err := BuildIgnoreDiffConfig(tc.ignoreDifferences, normalizers.IgnoreNormalizerOpts{})
			require.NoError(t, err)
			err = applyIgnoreDifferences(diffConfig, &foundApp, &generatedApp)
			require.NoError(t, err)
			yamlFound, err := yaml.Marshal(tc.foundApp)
			require.NoError(t, err)
			yamlExpected, err := yaml.Marshal(tc.expectedApp)
			require.NoError(t, err)
			assert.YAMLEq(t, string(yamlExpected), string(yamlFound))
		})
	}
}

type staleCacheClient struct {
	client.Client
	stale *v1alpha1.Application
}

func (c *staleCacheClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if app, ok := obj.(*v1alpha1.Application); ok && key.Name == c.stale.Name && key.Namespace == c.stale.Namespace {
		c.stale.DeepCopyInto(app)
		return nil
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

func (c *staleCacheClient) Patch(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
	return apierrors.NewNotFound(schema.GroupResource{Group: "argoproj.io", Resource: "applications"}, obj.GetName())
}

func TestCreateOrUpdate_PatchNotFoundFallsBackToCreate(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	stale := &v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.ApplicationSchemaGroupVersionKind.GroupVersion().String(),
			Kind:       v1alpha1.ApplicationSchemaGroupVersionKind.Kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			Namespace:       "argocd",
			ResourceVersion: "42",
		},
		Spec: v1alpha1.ApplicationSpec{Project: "old-project"},
	}

	underlying := fake.NewClientBuilder().WithScheme(scheme).Build()
	c := &staleCacheClient{Client: underlying, stale: stale}

	obj := &v1alpha1.Application{
		TypeMeta:   stale.TypeMeta,
		ObjectMeta: metav1.ObjectMeta{Name: stale.Name, Namespace: stale.Namespace},
	}
	desiredSpec := v1alpha1.ApplicationSpec{Project: "new-project"}
	action, err := CreateOrUpdate(context.Background(), log.NewEntry(log.New()), c, nil, obj, func() error {
		obj.Spec = desiredSpec
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, controllerutil.OperationResultCreated, action)

	var created v1alpha1.Application
	require.NoError(t, underlying.Get(context.Background(), client.ObjectKey{Name: stale.Name, Namespace: stale.Namespace}, &created))
	assert.Equal(t, "new-project", created.Spec.Project)
}
