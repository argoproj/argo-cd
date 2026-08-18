package utils

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/argo"
	argodiff "github.com/argoproj/argo-cd/v3/util/argo/diff"
	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
)

// CreateOrUpdate overrides "sigs.k8s.io/controller-runtime" function
// in sigs.k8s.io/controller-runtime/pkg/controller/controllerutil/controllerutil.go
// to add equality for argov1alpha1.ApplicationDestination
// argov1alpha1.ApplicationDestination has a private variable, so the default
// implementation fails to compare it.
//
// CreateOrUpdate creates or updates the given object in the Kubernetes
// cluster. The object's desired state must be reconciled with the existing
// state inside the passed in callback MutateFn.
//
// The MutateFn is called regardless of creating or updating an object.
//
// It returns the executed operation and an error.
// appEquality is the equality definition the write path uses when deciding whether to Patch.
// Anything asking "did the spec change?" must use the same one, or the two can disagree.
var appEquality = conversion.EqualitiesOrDie(
	func(a, b resource.Quantity) bool {
		// Ignore formatting, only care that numeric value stayed the same.
		// TODO: if we decide it's important, it should be safe to start comparing the format.
		//
		// Uninitialized quantities are equivalent to 0 quantities.
		return a.Cmp(b) == 0
	},
	func(a, b metav1.MicroTime) bool {
		return a.UTC().Equal(b.UTC())
	},
	func(a, b metav1.Time) bool {
		return a.UTC().Equal(b.UTC())
	},
	func(a, b labels.Selector) bool {
		return a.String() == b.String()
	},
	func(a, b fields.Selector) bool {
		return a.String() == b.String()
	},
	func(a, b argov1alpha1.ApplicationDestination) bool {
		return a.Namespace == b.Namespace && a.Name == b.Name && a.Server == b.Server
	},
)

func CreateOrUpdate(ctx context.Context, logCtx *log.Entry, c client.Client, ignoreAppDifferences argov1alpha1.ApplicationSetIgnoreDifferences, ignoreNormalizerOpts normalizers.IgnoreNormalizerOpts, obj *argov1alpha1.Application, f controllerutil.MutateFn) (controllerutil.OperationResult, error) {
	key := client.ObjectKeyFromObject(obj)
	if err := c.Get(ctx, key, obj); err != nil {
		if !errors.IsNotFound(err) {
			return controllerutil.OperationResultNone, err
		}
		if err := mutate(f, key, obj); err != nil {
			return controllerutil.OperationResultNone, err
		}
		if err := c.Create(ctx, obj); err != nil {
			return controllerutil.OperationResultNone, err
		}
		return controllerutil.OperationResultCreated, nil
	}

	normalizedLive := obj.DeepCopy()

	// Mutate the live object to match the desired state.
	if err := mutate(f, key, obj); err != nil {
		return controllerutil.OperationResultNone, err
	}

	// Apply ignoreApplicationDifferences rules to remove ignored fields from both the live and the desired state. This
	// prevents those differences from appearing in the diff and therefore in the patch.
	err := applyIgnoreDifferences(ignoreAppDifferences, normalizedLive, obj, ignoreNormalizerOpts)
	if err != nil {
		return controllerutil.OperationResultNone, fmt.Errorf("failed to apply ignore differences: %w", err)
	}

	// Normalize to avoid diffing on unimportant differences.
	normalizedLive.Spec = *argo.NormalizeApplicationSpec(&normalizedLive.Spec)
	obj.Spec = *argo.NormalizeApplicationSpec(&obj.Spec)

	// Note: if the informer cache holds a stale entry for an application that no longer exists on
	// the API server, DeepEqual may match against that stale entry and we skip Patch here. The
	// eviction in cacheSyncingClient only runs on NotFound from a write operation, so this edge
	// case is not covered and relies on Kubernetes propagating the delete event to the informer.
	if appEquality.DeepEqual(normalizedLive, obj) {
		return controllerutil.OperationResultNone, nil
	}

	patch := client.MergeFrom(normalizedLive)
	if log.IsLevelEnabled(log.DebugLevel) {
		LogPatch(logCtx, patch, obj)
	}
	if err := c.Patch(ctx, obj, patch); err != nil {
		return controllerutil.OperationResultNone, err
	}
	return controllerutil.OperationResultUpdated, nil
}

func LogPatch(logCtx *log.Entry, patch client.Patch, obj *argov1alpha1.Application) {
	patchBytes, err := patch.Data(obj)
	if err != nil {
		logCtx.Errorf("failed to generate patch: %v", err)
	}
	// Get the patch as a plain object so it is easier to work with in json logs.
	var patchObj map[string]any
	err = json.Unmarshal(patchBytes, &patchObj)
	if err != nil {
		logCtx.Errorf("failed to unmarshal patch: %v", err)
	}
	logCtx.WithField("patch", patchObj).Debug("patching application")
}

// mutate wraps a MutateFn and applies validation to its result
func mutate(f controllerutil.MutateFn, key client.ObjectKey, obj client.Object) error {
	if err := f(); err != nil {
		return fmt.Errorf("error while wrapping using MutateFn: %w", err)
	}
	if newKey := client.ObjectKeyFromObject(obj); key != newKey {
		return stderrors.New("MutateFn cannot mutate object name and/or object namespace")
	}
	return nil
}

// applyIgnoreDifferences applies the ignore differences rules to the found application. It modifies the applications in place.
// SpecsEquivalent reports whether the desired Application spec matches the live one, applying the
// same ignoreApplicationDifferences rules and normalization CreateOrUpdate applies before deciding
// whether to Patch. Callers asking "has the spec changed?" must use this rather than comparing specs
// directly: a difference the write path would not act on leaves the Application uncorrected and still
// reported as pending every reconcile. Scoped to the spec -- CreateOrUpdate compares whole objects, so
// a metadata-only change is patched without being reported here, which cannot loop.
//
// The ordering below is load bearing and deliberately differs from master. Here CreateOrUpdate applies
// the ignore rules before normalizing, and its caller has already normalized the generated spec, so
// when the rules run the desired side is normalized and the live side is not. A rule selecting on a
// value that only appears after normalization therefore matches one side only. Reproducing that
// asymmetry is what keeps this answer and the write path's in step; normalizing both up front does not,
// and TestSpecComparisonAgreesWithWritePath fails if this is changed to match master.
func SpecsEquivalent(ignoreDifferences argov1alpha1.ApplicationSetIgnoreDifferences, ignoreNormalizerOpts normalizers.IgnoreNormalizerOpts, live, desired *argov1alpha1.Application) (bool, error) {
	normalizedLive := live.DeepCopy()
	normalizedDesired := desired.DeepCopy()

	// As createOrUpdateInCluster does before calling CreateOrUpdate.
	normalizedDesired.Spec = *argo.NormalizeApplicationSpec(&normalizedDesired.Spec)

	if err := applyIgnoreDifferences(ignoreDifferences, normalizedLive, normalizedDesired, ignoreNormalizerOpts); err != nil {
		return false, fmt.Errorf("failed to apply ignore differences: %w", err)
	}

	normalizedLive.Spec = *argo.NormalizeApplicationSpec(&normalizedLive.Spec)
	normalizedDesired.Spec = *argo.NormalizeApplicationSpec(&normalizedDesired.Spec)

	return appEquality.DeepEqual(normalizedLive.Spec, normalizedDesired.Spec), nil
}

func applyIgnoreDifferences(applicationSetIgnoreDifferences argov1alpha1.ApplicationSetIgnoreDifferences, found *argov1alpha1.Application, generatedApp *argov1alpha1.Application, ignoreNormalizerOpts normalizers.IgnoreNormalizerOpts) error {
	if len(applicationSetIgnoreDifferences) == 0 {
		return nil
	}

	generatedAppCopy := generatedApp.DeepCopy()
	foundTypeMeta := found.TypeMeta
	diffConfig, err := argodiff.NewDiffConfigBuilder().
		WithDiffSettings(applicationSetIgnoreDifferences.ToApplicationIgnoreDifferences(), nil, false, ignoreNormalizerOpts).
		WithNoCache().
		Build()
	if err != nil {
		return fmt.Errorf("failed to build diff config: %w", err)
	}
	// diffConfig's rules are scoped to Group=argoproj.io, Kind=Application, but appToUnstructured goes
	// through runtime.DefaultUnstructuredConverter, which does not populate apiVersion/kind -- and the
	// live object arrives decoded from the API server while the generated one is built in Go with no
	// TypeMeta. Without stamping the GVK the rules match one side only, so an ignored field is
	// neutralised there and left intact on the other, and the two can never compare equal.
	foundForDiff := found.DeepCopy()
	foundForDiff.SetGroupVersionKind(argov1alpha1.ApplicationSchemaGroupVersionKind)
	generatedForDiff := generatedApp.DeepCopy()
	generatedForDiff.SetGroupVersionKind(argov1alpha1.ApplicationSchemaGroupVersionKind)

	unstructuredFound, err := appToUnstructured(foundForDiff)
	if err != nil {
		return fmt.Errorf("failed to convert found application to unstructured: %w", err)
	}
	unstructuredGenerated, err := appToUnstructured(generatedForDiff)
	if err != nil {
		return fmt.Errorf("failed to convert generated application to unstructured: %w", err)
	}
	result, err := argodiff.Normalize([]*unstructured.Unstructured{unstructuredFound}, []*unstructured.Unstructured{unstructuredGenerated}, diffConfig)
	if err != nil {
		return fmt.Errorf("failed to normalize application spec: %w", err)
	}
	if len(result.Lives) != 1 {
		return fmt.Errorf("expected 1 normalized application, got %d", len(result.Lives))
	}
	foundJSONNormalized, err := json.Marshal(result.Lives[0].Object)
	if err != nil {
		return fmt.Errorf("failed to marshal normalized app to json: %w", err)
	}
	foundNormalized := &argov1alpha1.Application{}
	err = json.Unmarshal(foundJSONNormalized, &foundNormalized)
	if err != nil {
		return fmt.Errorf("failed to unmarshal normalized app to json: %w", err)
	}
	if len(result.Targets) != 1 {
		return fmt.Errorf("expected 1 normalized application, got %d", len(result.Targets))
	}
	foundNormalized.DeepCopyInto(found)
	found.TypeMeta = foundTypeMeta
	generatedJSONNormalized, err := json.Marshal(result.Targets[0].Object)
	if err != nil {
		return fmt.Errorf("failed to marshal normalized app to json: %w", err)
	}
	generatedAppNormalized := &argov1alpha1.Application{}
	err = json.Unmarshal(generatedJSONNormalized, &generatedAppNormalized)
	if err != nil {
		return fmt.Errorf("failed to unmarshal normalized app json to structured app: %w", err)
	}
	generatedAppNormalized.DeepCopyInto(generatedApp)
	// Prohibit jq queries from mutating silly things.
	generatedApp.TypeMeta = generatedAppCopy.TypeMeta
	generatedApp.Name = generatedAppCopy.Name
	generatedApp.Namespace = generatedAppCopy.Namespace
	generatedApp.Operation = generatedAppCopy.Operation
	return nil
}

func appToUnstructured(app client.Object) (*unstructured.Unstructured, error) {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(app)
	if err != nil {
		return nil, fmt.Errorf("failed to convert app object to unstructured: %w", err)
	}
	return &unstructured.Unstructured{Object: u}, nil
}
