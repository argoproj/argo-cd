package utils

import (
	"context"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/argo"
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

// https://github.com/argoproj/argo-cd/issues/29066
//
// A tool like argocd-image-updater patches spec.source.kustomize.images directly onto the live
// Application. An ignoreApplicationDifferences rule on that path is supposed to make the
// ApplicationSet controller leave it alone. Removing the ignored field leaves an empty-but-present
// Kustomize struct on the live side (the generated side, built from a template with no kustomize
// block, has a nil pointer instead) unless that leftover is renormalized away — otherwise
// CreateOrUpdate sees a spurious diff and patches "kustomize": null over the image updater's write.
func TestCreateOrUpdateDoesNotRevertIgnoredKustomizeImages(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	live := &v1alpha1.Application{
		APIVersion: "argoproj.io/v1alpha1", Kind: "Application",
		Name: "demo-dev", Namespace: "argocd",
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Source: &v1alpha1.ApplicationSource{
				RepoURL: "https://github.com/argoproj/argocd-example-apps.git",
				Path:    "kustomize-guestbook",
				Kustomize: &v1alpha1.ApplicationSourceKustomize{
					Images: []v1alpha1.KustomizeImage{"gcr.io/heptio-images/ks-guestbook-demo:0.2"},
				},
			},
			Destination: v1alpha1.ApplicationDestination{
				Server: "https://kubernetes.default.svc", Namespace: "guestbook",
			},
		},
	}

	generated := live.DeepCopy()
	generated.Spec.Source.Kustomize = nil // template has no kustomize block
	generated.Spec = *argo.NormalizeApplicationSpec(&generated.Spec)

	ignore := v1alpha1.ApplicationSetIgnoreDifferences{
		{JSONPointers: []string{"/spec/source/kustomize/images"}},
	}
	diffConfig, err := BuildIgnoreDiffConfig(ignore, normalizers.IgnoreNormalizerOpts{})
	require.NoError(t, err)

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(live.DeepCopy()).Build()
	obj := live.DeepCopy()
	op, err := CreateOrUpdate(t.Context(), log.NewEntry(log.New()), c, diffConfig, obj, func() error {
		obj.Spec = generated.Spec
		return nil
	})
	require.NoError(t, err)

	require.Equal(t, controllerutil.OperationResultNone, op,
		"CreateOrUpdate must not patch when the only diff is on a field covered by ignoreApplicationDifferences")

	persisted := &v1alpha1.Application{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKeyFromObject(live), persisted))
	require.NotNil(t, persisted.Spec.Source.Kustomize, "the ignored kustomize.images override must survive in the cluster")
	require.Equal(t, live.Spec.Source.Kustomize.Images, persisted.Spec.Source.Kustomize.Images)
}

// https://github.com/argoproj/argo-cd/issues/29066
//
// Same scenario as TestCreateOrUpdateDoesNotRevertIgnoredKustomizeImages, but for a multi-source
// Application: argocd-image-updater patches spec.sources[N].kustomize.images and the
// ignoreApplicationDifferences rule targets that path. The renormalization must collapse the
// empty-but-present Kustomize struct left behind on the live side for the sources[] path too,
// otherwise CreateOrUpdate patches "kustomize": null over the image updater's write.
func TestCreateOrUpdateDoesNotRevertIgnoredKustomizeImagesMultiSource(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	live := &v1alpha1.Application{
		APIVersion: "argoproj.io/v1alpha1", Kind: "Application",
		Name: "demo-dev", Namespace: "argocd",
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Sources: v1alpha1.ApplicationSources{
				{
					RepoURL: "https://github.com/argoproj/argocd-example-apps.git",
					Path:    "helm-guestbook",
				},
				{
					RepoURL: "https://github.com/argoproj/argocd-example-apps.git",
					Path:    "kustomize-guestbook",
					Kustomize: &v1alpha1.ApplicationSourceKustomize{
						Images: []v1alpha1.KustomizeImage{"gcr.io/heptio-images/ks-guestbook-demo:0.2"},
					},
				},
			},
			Destination: v1alpha1.ApplicationDestination{
				Server: "https://kubernetes.default.svc", Namespace: "guestbook",
			},
		},
	}

	generated := live.DeepCopy()
	generated.Spec.Sources[1].Kustomize = nil // template has no kustomize block on this source
	generated.Spec = *argo.NormalizeApplicationSpec(&generated.Spec)

	ignore := v1alpha1.ApplicationSetIgnoreDifferences{
		{JSONPointers: []string{"/spec/sources/1/kustomize/images"}},
	}
	diffConfig, err := BuildIgnoreDiffConfig(ignore, normalizers.IgnoreNormalizerOpts{})
	require.NoError(t, err)

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(live.DeepCopy()).Build()
	obj := live.DeepCopy()
	op, err := CreateOrUpdate(t.Context(), log.NewEntry(log.New()), c, diffConfig, obj, func() error {
		obj.Spec = generated.Spec
		return nil
	})
	require.NoError(t, err)

	require.Equal(t, controllerutil.OperationResultNone, op,
		"CreateOrUpdate must not patch when the only diff is on a field covered by ignoreApplicationDifferences")

	persisted := &v1alpha1.Application{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKeyFromObject(live), persisted))
	require.NotNil(t, persisted.Spec.Sources[1].Kustomize, "the ignored kustomize.images override must survive in the cluster")
	require.Equal(t, live.Spec.Sources[1].Kustomize.Images, persisted.Spec.Sources[1].Kustomize.Images)
}

// staleGetClient returns an older snapshot from Get while Patch/Create apply to
// the inner client's stored object. Models AppSet's cache-backed Get lagging
// the API server (spec Test 1).
type staleGetClient struct {
	client.Client
	stale *v1alpha1.Application
}

func (s staleGetClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	app, ok := obj.(*v1alpha1.Application)
	if !ok {
		return s.Client.Get(ctx, key, obj, opts...)
	}
	s.stale.DeepCopyInto(app)
	return nil
}

func testApp(name string, op *v1alpha1.Operation) *v1alpha1.Application {
	app := &v1alpha1.Application{
		Spec:      v1alpha1.ApplicationSpec{Project: "default"},
		Operation: op,
	}
	app.Name = name
	app.Namespace = "argocd"
	return app
}

func fullAppSetOp() *v1alpha1.Operation {
	return &v1alpha1.Operation{
		InitiatedBy: v1alpha1.OperationInitiator{Username: AppSetControllerUsername, Automated: true},
		Info: []*v1alpha1.Info{{
			Name:  "Reason",
			Value: "ApplicationSet RollingSync triggered a sync of this Application resource",
		}},
		Sync:  &v1alpha1.SyncOperation{},
		Retry: v1alpha1.RetryStrategy{Limit: 5},
	}
}

func filteredOp(username string) *v1alpha1.Operation {
	return &v1alpha1.Operation{
		InitiatedBy: v1alpha1.OperationInitiator{Username: username, Automated: username == AppSetControllerUsername},
		Sync: &v1alpha1.SyncOperation{
			Resources: []v1alpha1.SyncOperationResource{{
				Group: "batch", Kind: "Job", Name: "cilium-post-sync", Namespace: "kube-system",
			}},
		},
	}
}

// Covers the stale-Get window (Operation==nil in cache while the API already has a
// filtered op): AppSet must overwrite. That is not the user-op preserve guarantee
// (OperationGuard case 5).
func TestCreateOrUpdate_StaleBaseClearsFilteredOperation(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	stored := testApp("demo", filteredOp("admin"))
	stale := testApp("demo", nil) // Get predates the filtered op

	inner := fake.NewClientBuilder().WithScheme(scheme).WithObjects(stored.DeepCopy()).Build()
	c := staleGetClient{Client: inner, stale: stale}

	obj := testApp("demo", nil)
	logCtx := log.NewEntry(log.New())
	_, err := CreateOrUpdate(t.Context(), logCtx, c, nil, obj, func() error {
		obj.Spec = stored.Spec
		obj.Operation = fullAppSetOp()
		return nil
	})
	require.NoError(t, err)

	got := &v1alpha1.Application{}
	require.NoError(t, inner.Get(t.Context(), client.ObjectKeyFromObject(stored), got))
	require.NotNil(t, got.Operation)
	require.NotNil(t, got.Operation.Sync)
	assert.Empty(t, got.Operation.Sync.Resources)
	assert.Equal(t, AppSetControllerUsername, got.Operation.InitiatedBy.Username)
}

func TestCreateOrUpdate_OperationGuard(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	tests := []struct {
		name           string
		liveOp         *v1alpha1.Operation
		desiredOp      *v1alpha1.Operation
		wantUser       string
		wantRes        int
		wantResult     controllerutil.OperationResult
		checkResult    bool
		wantRetryLimit int64
		checkRetry     bool
	}{
		{
			name:      "2 live nil generated set -> live gets generated",
			liveOp:    nil,
			desiredOp: fullAppSetOp(),
			wantUser:  AppSetControllerUsername,
			wantRes:   0,
		},
		{
			name:      "3 generated nil live user op -> live user op unchanged",
			liveOp:    filteredOp("admin"),
			desiredOp: nil,
			wantUser:  "admin",
			wantRes:   1,
		},
		{
			name:      "4 live AppSet-initiated filtered + generated full -> filter gone",
			liveOp:    filteredOp(AppSetControllerUsername),
			desiredOp: fullAppSetOp(),
			wantUser:  AppSetControllerUsername,
			wantRes:   0,
		},
		{
			name:      "5 live user-initiated + generated full -> live user op unchanged",
			liveOp:    filteredOp("admin"),
			desiredOp: fullAppSetOp(),
			wantUser:  "admin",
			wantRes:   1,
		},
		{
			name:   "6 live already-full AppSet op + generated full -> do not re-add",
			liveOp: fullAppSetOp(),
			desiredOp: func() *v1alpha1.Operation {
				op := fullAppSetOp()
				op.Retry.Limit = 9
				return op
			}(),
			wantUser:       AppSetControllerUsername,
			wantRes:        0,
			wantResult:     controllerutil.OperationResultNone,
			checkResult:    true,
			wantRetryLimit: 5,
			checkRetry:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			live := testApp("demo", tc.liveOp)
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(live.DeepCopy()).Build()
			obj := testApp("demo", nil)
			result, err := CreateOrUpdate(t.Context(), log.NewEntry(log.New()), c, nil, obj, func() error {
				obj.Spec = live.Spec
				if tc.desiredOp != nil {
					obj.Operation = tc.desiredOp
				}
				return nil
			})
			require.NoError(t, err)
			if tc.checkResult {
				assert.Equal(t, tc.wantResult, result)
			}

			got := &v1alpha1.Application{}
			require.NoError(t, c.Get(t.Context(), client.ObjectKeyFromObject(live), got))
			require.NotNil(t, got.Operation)
			assert.Equal(t, tc.wantUser, got.Operation.InitiatedBy.Username)
			require.NotNil(t, got.Operation.Sync)
			assert.Len(t, got.Operation.Sync.Resources, tc.wantRes)
			if tc.checkRetry {
				assert.Equal(t, tc.wantRetryLimit, got.Operation.Retry.Limit)
			}
		})
	}
}
