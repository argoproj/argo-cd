package utils

import (
	"context"
	"errors"
	"fmt"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8scache "k8s.io/client-go/tools/cache"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	application "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
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

// informerBackedClient simulates the manager's cache-backed client in production: reads are
// served from the informer store, while writes (and everything else) go to the wrapped API
// client.
type informerBackedClient struct {
	client.Client
	store k8scache.Store
}

func (c *informerBackedClient) Get(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	item, exists, err := c.store.GetByKey(key.String())
	if err != nil {
		return err
	}
	if !exists {
		return apierrors.NewNotFound(schema.GroupResource{Group: "argoproj.io", Resource: "applications"}, key.Name)
	}
	app, ok := item.(*application.Application)
	if !ok {
		return fmt.Errorf("informer store holds unexpected type %T", item)
	}
	app.DeepCopyInto(obj.(*application.Application))
	return nil
}

func mustScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, application.AddToScheme(scheme))
	return scheme
}

func mustStoreWith(t *testing.T, objs ...client.Object) k8scache.Store {
	t.Helper()
	store := k8scache.NewStore(k8scache.MetaNamespaceKeyFunc)
	for _, obj := range objs {
		require.NoError(t, store.Add(obj))
	}
	return store
}

// newCacheSyncingClient builds a cacheSyncingClient whose reads come from an informer store
// seeded with cachedObjects, whose writes go to apiClient (the fake API server), and whose
// uncached reader is also apiClient. This reproduces the production wiring
// (NewCacheSyncingClient(mgr.GetClient(), mgr.GetCache(), mgr.GetAPIReader())).
func newCacheSyncingClient(t *testing.T, apiClient client.Client, cachedObjects ...client.Object) (*cacheSyncingClient, k8scache.Store) {
	t.Helper()
	store := mustStoreWith(t, cachedObjects...)
	c := &cacheSyncingClient{
		Client:     &informerBackedClient{Client: apiClient, store: store},
		apiReader:  apiClient,
		storesByNs: map[string]k8scache.Store{},
		getNSCache: func(_ context.Context, _ client.Object) (ctrlcache.Cache, error) {
			return &fakeMultiNamespaceCache{Store: store}, nil
		},
	}
	return c, store
}

func staleCacheGhostApp() *application.Application {
	app := &application.Application{
		Spec: application.ApplicationSpec{
			Project:    "project",
			SyncPolicy: &application.SyncPolicy{},
		},
	}
	app.Name = "app1"
	app.Namespace = "argocd"
	app.ResourceVersion = "7"
	return app
}

func desiredApp() *application.Application {
	app := &application.Application{}
	app.Name = "app1"
	app.Namespace = "argocd"
	return app
}

// TestCreateOrUpdateRecreatesAppDeletedOutOfBand is the regression test for issue #30848: the
// informer cache still holds an entry for an Application that was deleted directly on the API
// server, and the cached object matches the desired state, so the DeepEqual check alone would
// skip the write and the Application would never be recreated. The controller must confirm the
// cache hit against the API server, evict the stale entry and recreate the Application.
func TestCreateOrUpdateRecreatesAppDeletedOutOfBand(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	ghost := staleCacheGhostApp()
	// The API server does NOT contain the Application; only the informer store does.
	apiClient := fake.NewClientBuilder().WithScheme(mustScheme(t)).Build()
	c, store := newCacheSyncingClient(t, apiClient, ghost)

	desired := desiredApp()
	action, err := CreateOrUpdate(ctx, log.NewEntry(log.StandardLogger()), c, nil, desired, func() error {
		desired.Spec = application.ApplicationSpec{Project: "project"}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, controllerutil.OperationResultCreated, action, "the stale cache entry must trigger recreation, not a silent skip")

	// The Application exists again on the API server.
	got := &application.Application{}
	require.NoError(t, apiClient.Get(ctx, client.ObjectKey{Name: "app1", Namespace: "argocd"}, got))
	require.Equal(t, "project", got.Spec.Project)

	// The informer store was healed: the stale entry was evicted and the recreated Application
	// synced back in by the Create.
	item, exists, err := store.GetByKey("argocd/app1")
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, "project", item.(*application.Application).Spec.Project)
}

// TestCreateOrUpdateSkipsUnchangedAppConfirmedByAPI guards the normal path: when the informer
// cache entry is backed by a real object on the API server, an unchanged Application is skipped
// without any write.
func TestCreateOrUpdateSkipsUnchangedAppConfirmedByAPI(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	live := staleCacheGhostApp()
	live.ResourceVersion = "5"

	var writes int
	apiClient := fake.NewClientBuilder().WithScheme(mustScheme(t)).WithObjects(live.DeepCopy()).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				writes++
				return c.Create(ctx, obj, opts...)
			},
			Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				writes++
				return c.Patch(ctx, obj, patch, opts...)
			},
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				writes++
				return c.Update(ctx, obj, opts...)
			},
		}).Build()

	// Both the informer store and the API server hold the Application.
	c, _ := newCacheSyncingClient(t, apiClient, live)

	desired := desiredApp()
	action, err := CreateOrUpdate(ctx, log.NewEntry(log.StandardLogger()), c, nil, desired, func() error {
		desired.Spec = application.ApplicationSpec{Project: "project"}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, controllerutil.OperationResultNone, action)
	require.Zero(t, writes, "an unchanged Application confirmed on the API server must not be written")

	got := &application.Application{}
	require.NoError(t, apiClient.Get(ctx, client.ObjectKey{Name: "app1", Namespace: "argocd"}, got))
	require.Equal(t, "5", got.ResourceVersion, "no write should have bumped the resource version")
}

// TestCreateOrUpdateProbeErrorFallsBackToCache ensures a transient error from the uncached read
// does not fail the reconcile: the controller falls back to trusting the informer cache.
func TestCreateOrUpdateProbeErrorFallsBackToCache(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	ghost := staleCacheGhostApp()
	probeErr := errors.New("connection refused")
	apiClient := fake.NewClientBuilder().WithScheme(mustScheme(t)).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
				return probeErr
			},
		}).Build()
	c, _ := newCacheSyncingClient(t, apiClient, ghost)

	desired := desiredApp()
	action, err := CreateOrUpdate(ctx, log.NewEntry(log.StandardLogger()), c, nil, desired, func() error {
		desired.Spec = application.ApplicationSpec{Project: "project"}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, controllerutil.OperationResultNone, action)

	// Nothing was created behind the API client's back.
	require.Error(t, apiClient.Get(ctx, client.ObjectKey{Name: "app1", Namespace: "argocd"}, &application.Application{}))
}

// TestCreateOrUpdateWithoutVerifierKeepsOldBehavior: a plain client (no uncached reader, e.g. in
// tests that use the fake client directly) keeps the previous behavior and skips the write.
func TestCreateOrUpdateWithoutVerifierKeepsOldBehavior(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	live := staleCacheGhostApp()
	apiClient := fake.NewClientBuilder().WithScheme(mustScheme(t)).WithObjects(live.DeepCopy()).Build()
	c, _ := newCacheSyncingClient(t, apiClient, live)
	c.apiReader = nil // no uncached reader: the cache cannot be verified

	desired := desiredApp()
	action, err := CreateOrUpdate(ctx, log.NewEntry(log.StandardLogger()), c, nil, desired, func() error {
		desired.Spec = application.ApplicationSpec{Project: "project"}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, controllerutil.OperationResultNone, action)
}

// TestCreateOrUpdateRecreateDropsInheritedCachedOperation guards the recreation path: an
// Operation present on the stale cache entry but not requested by the template must not be
// persisted on the recreated Application, otherwise an old sync request would run again.
func TestCreateOrUpdateRecreateDropsInheritedCachedOperation(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	ghost := staleCacheGhostApp()
	ghost.Operation = &application.Operation{Sync: &application.SyncOperation{}}
	apiClient := fake.NewClientBuilder().WithScheme(mustScheme(t)).Build()
	c, _ := newCacheSyncingClient(t, apiClient, ghost)

	desired := desiredApp()
	action, err := CreateOrUpdate(ctx, log.NewEntry(log.StandardLogger()), c, nil, desired, func() error {
		desired.Spec = application.ApplicationSpec{Project: "project"}
		// the template does NOT request an Operation
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, controllerutil.OperationResultCreated, action)

	got := &application.Application{}
	require.NoError(t, apiClient.Get(ctx, client.ObjectKey{Name: "app1", Namespace: "argocd"}, got))
	require.Nil(t, got.Operation, "an Operation inherited from the stale cache entry must not be persisted on the recreated Application")
}

// TestCreateOrUpdateRecreateKeepsTemplateOperation: when the template explicitly requests an
// Operation, recreation must carry it over.
func TestCreateOrUpdateRecreateKeepsTemplateOperation(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	ghost := staleCacheGhostApp()
	apiClient := fake.NewClientBuilder().WithScheme(mustScheme(t)).Build()
	c, _ := newCacheSyncingClient(t, apiClient, ghost)

	desired := desiredApp()
	action, err := CreateOrUpdate(ctx, log.NewEntry(log.StandardLogger()), c, nil, desired, func() error {
		desired.Spec = application.ApplicationSpec{Project: "project"}
		desired.Operation = &application.Operation{Sync: &application.SyncOperation{}}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, controllerutil.OperationResultCreated, action)

	got := &application.Application{}
	require.NoError(t, apiClient.Get(ctx, client.ObjectKey{Name: "app1", Namespace: "argocd"}, got))
	require.NotNil(t, got.Operation, "a template-requested Operation must survive recreation")
}

// TestCreateOrUpdateRecreateKeepsIgnoredTargetRevision guards the recreation path against
// ignore-difference normalization: the Create must be built from the desired state captured
// before applyIgnoreDifferences removed ignored fields, so e.g. a non-empty targetRevision is
// not lost on the recreated Application.
func TestCreateOrUpdateRecreateKeepsIgnoredTargetRevision(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	ghost := staleCacheGhostApp()
	ghost.Spec.Source = &application.ApplicationSource{RepoURL: "https://example.com/repo.git", TargetRevision: "old"}
	apiClient := fake.NewClientBuilder().WithScheme(mustScheme(t)).Build()
	c, _ := newCacheSyncingClient(t, apiClient, ghost)

	ignore := v1alpha1.ApplicationSetIgnoreDifferences{{JSONPointers: []string{"/spec/source/targetRevision"}}}
	diffConfig, err := BuildIgnoreDiffConfig(ignore, normalizers.IgnoreNormalizerOpts{})
	require.NoError(t, err)

	desired := desiredApp()
	action, err := CreateOrUpdate(ctx, log.NewEntry(log.StandardLogger()), c, diffConfig, desired, func() error {
		desired.Spec = application.ApplicationSpec{
			Project: "project",
			Source:  &application.ApplicationSource{RepoURL: "https://example.com/repo.git", TargetRevision: "main"},
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, controllerutil.OperationResultCreated, action)

	got := &application.Application{}
	require.NoError(t, apiClient.Get(ctx, client.ObjectKey{Name: "app1", Namespace: "argocd"}, got))
	require.Equal(t, "main", got.Spec.Source.TargetRevision, "ignored fields must survive recreation")
}
