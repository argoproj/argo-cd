/*
The package provide functions that allows to compare set of Kubernetes resources using the logic equivalent to
`kubectl diff`.
*/
package diff

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	jsonpatch "github.com/evanphx/json-patch/v5"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/jsonmergepatch"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
	"sigs.k8s.io/structured-merge-diff/v4/merge"
	"sigs.k8s.io/structured-merge-diff/v4/typed"

	"github.com/argoproj/gitops-engine/internal/kubernetes_vendor/pkg/api/v1/endpoints"
	"github.com/argoproj/gitops-engine/pkg/diff/internal/fieldmanager"
	"github.com/argoproj/gitops-engine/pkg/sync/resource"
	jsonutil "github.com/argoproj/gitops-engine/pkg/utils/json"
	gescheme "github.com/argoproj/gitops-engine/pkg/utils/kube/scheme"
)

const (
	couldNotMarshalErrMsg       = "Could not unmarshal to object of type %s: %v"
	AnnotationLastAppliedConfig = "kubectl.kubernetes.io/last-applied-configuration"
	replacement                 = "++++++++"
)

// Holds diffing result of two resources
type DiffResult struct {
	// Modified is set to true if resources are not matching
	Modified bool
	// Contains YAML representation of a live resource with applied normalizations
	NormalizedLive []byte
	// Contains "expected" YAML representation of a live resource
	PredictedLive []byte
}

// Holds result of two resources sets comparison
type DiffResultList struct {
	Diffs    []DiffResult
	Modified bool
}

type noopNormalizer struct{}

func (n *noopNormalizer) Normalize(_ *unstructured.Unstructured) error {
	return nil
}

// Normalizer updates resource before comparing it
type Normalizer interface {
	Normalize(un *unstructured.Unstructured) error
}

// GetNoopNormalizer returns normalizer that does not apply any resource modifications
func GetNoopNormalizer() Normalizer {
	return &noopNormalizer{}
}

// Diff performs a diff on two unstructured objects. If the live object happens to have a
// "kubectl.kubernetes.io/last-applied-configuration", then perform a three way diff.
func Diff(config, live *unstructured.Unstructured, opts ...Option) (*DiffResult, error) {
	o := applyOptions(opts)
	if config != nil {
		config = remarshal(config, o)
		Normalize(config, opts...)
	}
	if live != nil {
		live = remarshal(live, o)
		Normalize(live, opts...)
	}

	if o.serverSideDiff {
		r, err := ServerSideDiff(config, live, opts...)
		if err != nil {
			return nil, fmt.Errorf("error calculating server side diff: %w", err)
		}
		return r, nil
	}

	// TODO The two variables bellow are necessary because there is a cyclic
	// dependency with the kube package that blocks the usage of constants
	// from common package. common package needs to be refactored and exclude
	// dependency from kube.
	syncOptAnnotation := "argocd.argoproj.io/sync-options"
	ssaAnnotation := "ServerSideApply=true"

	// structuredMergeDiff is mainly used as a feature flag to enable
	// calculating diffs using the structured-merge-diff library
	// used in k8s while performing server-side applies. It checks the
	// given diff Option or if the desired state resource has the
	// Server-Side apply sync option annotation enabled.
	structuredMergeDiff := o.structuredMergeDiff ||
		(config != nil && resource.HasAnnotationOption(config, syncOptAnnotation, ssaAnnotation))
	if structuredMergeDiff {
		r, err := StructuredMergeDiff(config, live, o.gvkParser, o.manager)
		if err != nil {
			return nil, fmt.Errorf("error calculating structured merge diff: %w", err)
		}
		return r, nil
	}
	orig, err := GetLastAppliedConfigAnnotation(live)
	if err != nil {
		o.log.V(1).Info(fmt.Sprintf("Failed to get last applied configuration: %v", err))
	} else if orig != nil && config != nil {
		Normalize(orig, opts...)
		dr, err := ThreeWayDiff(orig, config, live)
		if err == nil {
			return dr, nil
		}
		o.log.V(1).Info(fmt.Sprintf("three-way diff calculation failed: %v. Falling back to two-way diff", err))
	}
	return TwoWayDiff(config, live)
}

// ServerSideDiff will execute a k8s server-side apply in dry-run mode with the
// given config. The result will be compared with given live resource to determine
// diff. If config or live are nil it means resource creation or deletion. In this
// no call will be made to kube-api and a simple diff will be returned.
func ServerSideDiff(config, live *unstructured.Unstructured, opts ...Option) (*DiffResult, error) {
	if live != nil && config != nil {
		result, err := serverSideDiff(config, live, opts...)
		if err != nil {
			return nil, fmt.Errorf("serverSideDiff error: %w", err)
		}
		return result, nil
	}
	// Currently, during resource creation a shallow diff (non ServerSide apply
	// based) will be returned. The reasons are:
	// - Saves 1 additional call to KubeAPI
	// - Much lighter/faster diff
	// - This is the existing behaviour users are already used to
	// - No direct benefit to the user
	result, err := handleResourceCreateOrDeleteDiff(config, live)
	if err != nil {
		return nil, fmt.Errorf("error handling resource creation or deletion: %w", err)
	}
	return result, nil
}

// ServerSideDiff will execute a k8s server-side apply in dry-run mode with the
// given config. The result will be compared with given live resource to determine
// diff. Modifications done by mutation webhooks are removed from the diff by default.
// This behaviour can be customized with Option.WithIgnoreMutationWebhook.
func serverSideDiff(config, live *unstructured.Unstructured, opts ...Option) (*DiffResult, error) {
	o := applyOptions(opts)
	if o.serverSideDryRunner == nil {
		return nil, errors.New("serverSideDryRunner is null")
	}
	predictedLiveStr, err := o.serverSideDryRunner.Run(context.Background(), config, o.manager)
	if err != nil {
		return nil, fmt.Errorf("error running server side apply in dryrun mode for resource %s/%s: %w", config.GetKind(), config.GetName(), err)
	}
	predictedLive, err := jsonStrToUnstructured(predictedLiveStr)
	if err != nil {
		return nil, fmt.Errorf("error converting json string to unstructured for resource %s/%s: %w", config.GetKind(), config.GetName(), err)
	}

	if o.ignoreMutationWebhook {
		predictedLive, err = removeWebhookMutation(predictedLive, live, o.gvkParser, o.manager)
		if err != nil {
			return nil, fmt.Errorf("error removing non config mutations for resource %s/%s: %w", config.GetKind(), config.GetName(), err)
		}
	}

	Normalize(predictedLive, opts...)
	unstructured.RemoveNestedField(predictedLive.Object, "metadata", "managedFields")

	predictedLiveBytes, err := json.Marshal(predictedLive)
	if err != nil {
		return nil, fmt.Errorf("error marshaling predicted live for resource %s/%s: %w", config.GetKind(), config.GetName(), err)
	}

	unstructured.RemoveNestedField(live.Object, "metadata", "managedFields")
	liveBytes, err := json.Marshal(live)
	if err != nil {
		return nil, fmt.Errorf("error marshaling live resource %s/%s: %w", config.GetKind(), config.GetName(), err)
	}
	return buildDiffResult(predictedLiveBytes, liveBytes), nil
}

// removeWebhookMutation will compare the predictedLive with live to identify changes done by mutation webhooks.
// Webhook mutations are removed from predictedLive by removing all fields which are not managed by the given 'manager'.
// At this step, we will only have the fields that are managed by the given 'manager'.
// It is then merged with the live state and re-assigned to predictedLive. This means that any
// fields not managed by the specified manager will be reverted with their state from live, including any webhook mutations.
// If the given predictedLive does not have the managedFields, an error will be returned.
func removeWebhookMutation(predictedLive, live *unstructured.Unstructured, gvkParser *managedfields.GvkParser, manager string) (*unstructured.Unstructured, error) {
	plManagedFields := predictedLive.GetManagedFields()
	if len(plManagedFields) == 0 {
		return nil, fmt.Errorf("predictedLive for resource %s/%s must have the managedFields", predictedLive.GetKind(), predictedLive.GetName())
	}
	gvk := predictedLive.GetObjectKind().GroupVersionKind()
	pt := gvkParser.Type(gvk)
	if pt == nil {
		return nil, fmt.Errorf("unable to resolve parseableType for GroupVersionKind: %s", gvk)
	}

	typedPredictedLive, err := pt.FromUnstructured(predictedLive.Object)
	if err != nil {
		return nil, fmt.Errorf("error converting predicted live state from unstructured to %s: %w", gvk, err)
	}

	typedLive, err := pt.FromUnstructured(live.Object)
	if err != nil {
		return nil, fmt.Errorf("error converting live state from unstructured to %s: %w", gvk, err)
	}

	// Initialize an empty fieldpath.Set to aggregate managed fields for the specified manager
	managerFieldsSet := &fieldpath.Set{}

	// Iterate over all ManagedFields entries in predictedLive
	for _, mfEntry := range plManagedFields {
		managedFieldsSet := &fieldpath.Set{}
		err := managedFieldsSet.FromJSON(bytes.NewReader(mfEntry.FieldsV1.Raw))
		if err != nil {
			return nil, fmt.Errorf("error building managedFields set: %w", err)
		}
		if mfEntry.Manager == manager {
			// Union the fields with the aggregated set
			managerFieldsSet = managerFieldsSet.Union(managedFieldsSet)
		}
	}

	if managerFieldsSet.Empty() {
		return nil, fmt.Errorf("no managed fields found for manager: %s", manager)
	}

	predictedLiveFieldSet, err := typedPredictedLive.ToFieldSet()
	if err != nil {
		return nil, fmt.Errorf("error converting predicted live state to FieldSet: %w", err)
	}

	// Remove fields from predicted live that are not managed by the provided manager
	nonArgoFieldsSet := predictedLiveFieldSet.Difference(managerFieldsSet)

	// In case any of the removed fields cause schema violations, we will keep those fields
	nonArgoFieldsSet = safelyRemoveFieldsSet(typedPredictedLive, nonArgoFieldsSet)
	typedPredictedLive = typedPredictedLive.RemoveItems(nonArgoFieldsSet)

	// Apply the predicted live state to the live state to get a diff without mutation webhook fields
	typedPredictedLive, err = typedLive.Merge(typedPredictedLive)
	if err != nil {
		return nil, fmt.Errorf("error applying predicted live to live state: %w", err)
	}

	plu := typedPredictedLive.AsValue().Unstructured()
	pl, ok := plu.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("error converting live typedValue: expected map got %T", plu)
	}
	return &unstructured.Unstructured{Object: pl}, nil
}

// safelyRemoveFieldSet will validate if removing the fieldsToRemove set from predictedLive maintains
// a valid schema. If removing a field in fieldsToRemove is invalid and breaks the schema, it is not safe
// to remove and will be skipped from removal from predictedLive.
func safelyRemoveFieldsSet(predictedLive *typed.TypedValue, fieldsToRemove *fieldpath.Set) *fieldpath.Set {
	// In some cases, we cannot remove fields due to violation of the predicted live schema. In such cases we validate the removal
	// of each field and only include it if the removal is valid.
	testPredictedLive := predictedLive.RemoveItems(fieldsToRemove)
	err := testPredictedLive.Validate()
	if err != nil {
		adjustedFieldsToRemove := fieldpath.NewSet()
		fieldsToRemove.Iterate(func(p fieldpath.Path) {
			singleFieldSet := fieldpath.NewSet(p)
			testSingleRemoval := predictedLive.RemoveItems(singleFieldSet)
			// Check if removing this single field maintains a valid schema
			if testSingleRemoval.Validate() == nil {
				// If valid, add this field to the adjusted set to remove
				adjustedFieldsToRemove.Insert(p)
			}
		})
		return adjustedFieldsToRemove
	}
	// If no violations, return the original set to remove
	return fieldsToRemove
}

func jsonStrToUnstructured(jsonString string) (*unstructured.Unstructured, error) {
	res := make(map[string]any)
	err := json.Unmarshal([]byte(jsonString), &res)
	if err != nil {
		return nil, fmt.Errorf("unmarshal error: %w", err)
	}
	return &unstructured.Unstructured{Object: res}, nil
}

// StructuredMergeDiff will calculate the diff using the structured-merge-diff
// k8s library (https://github.com/kubernetes-sigs/structured-merge-diff).
func StructuredMergeDiff(config, live *unstructured.Unstructured, gvkParser *managedfields.GvkParser, manager string) (*DiffResult, error) {
	if live != nil && config != nil {
		params := &SMDParams{
			config:    config,
			live:      live,
			gvkParser: gvkParser,
			manager:   manager,
		}
		return structuredMergeDiff(params)
	}
	return handleResourceCreateOrDeleteDiff(config, live)
}

// SMDParams defines the parameters required by the structuredMergeDiff
// function
type SMDParams struct {
	config    *unstructured.Unstructured
	live      *unstructured.Unstructured
	gvkParser *managedfields.GvkParser
	manager   string
}

func structuredMergeDiff(p *SMDParams) (*DiffResult, error) {
	gvk := p.config.GetObjectKind().GroupVersionKind()
	pt := gescheme.ResolveParseableType(gvk, p.gvkParser)
	if pt == nil {
		return nil, fmt.Errorf("unable to resolve parseableType for GroupVersionKind: %s", gvk)
	}

	// Build typed value from live and config unstructures
	tvLive, err := pt.FromUnstructured(p.live.Object)
	if err != nil {
		return nil, fmt.Errorf("error building typed value from live resource: %w", err)
	}
	tvConfig, err := pt.FromUnstructured(p.config.Object)
	if err != nil {
		return nil, fmt.Errorf("error building typed value from config resource: %w", err)
	}

	// Invoke the apply function to calculate the diff using
	// the structured-merge-diff library
	mergedLive, err := apply(tvConfig, tvLive, p)
	if err != nil {
		return nil, fmt.Errorf("error calculating diff: %w", err)
	}

	// When mergedLive is nil it means that there is no change
	if mergedLive == nil {
		liveBytes, err := json.Marshal(p.live)
		if err != nil {
			return nil, fmt.Errorf("error marshaling live resource: %w", err)
		}
		// In this case diff result will have live state for both,
		// predicted and live.
		return buildDiffResult(liveBytes, liveBytes), nil
	}

	// Normalize merged live
	predictedLive, err := normalizeTypedValue(mergedLive)
	if err != nil {
		return nil, fmt.Errorf("error applying default values in predicted live: %w", err)
	}

	// Normalize live
	taintedLive, err := normalizeTypedValue(tvLive)
	if err != nil {
		return nil, fmt.Errorf("error applying default values in live: %w", err)
	}

	return buildDiffResult(predictedLive, taintedLive), nil
}

// apply will build all the dependency required to invoke the smd.merge.updater.Apply
// to correctly calculate the diff with the same logic used in k8s with server-side
// apply.
func apply(tvConfig, tvLive *typed.TypedValue, p *SMDParams) (*typed.TypedValue, error) {
	// Build the structured-merge-diff Updater
	updater := merge.Updater{
		Converter: fieldmanager.NewVersionConverter(p.gvkParser, scheme.Scheme, p.config.GroupVersionKind().GroupVersion()),
	}

	// Build a list of managers and which API version they own
	managed, err := fieldmanager.DecodeManagedFields(p.live.GetManagedFields())
	if err != nil {
		return nil, fmt.Errorf("error decoding managed fields: %w", err)
	}

	// Use the desired manifest to extract the target resource version
	version := fieldpath.APIVersion(p.config.GetAPIVersion())

	// The manager string needs to be converted to the internal manager
	// key used inside structured-merge-diff apply logic
	managerKey, err := buildManagerInfoForApply(p.manager)
	if err != nil {
		return nil, fmt.Errorf("error building manager info: %w", err)
	}

	// Finally invoke Apply to execute the same function used in k8s
	// server-side applies
	mergedLive, _, err := updater.Apply(tvLive, tvConfig, version, managed.Fields(), managerKey, true)
	if err != nil {
		return nil, fmt.Errorf("error while running updater.Apply: %w", err)
	}
	return mergedLive, err
}

func buildManagerInfoForApply(manager string) (string, error) {
	managerInfo := metav1.ManagedFieldsEntry{
		Manager:   manager,
		Operation: metav1.ManagedFieldsOperationApply,
	}
	return fieldmanager.BuildManagerIdentifier(&managerInfo)
}

// normalizeTypedValue will prepare the given tv so it can be used in diffs by:
// - removing last-applied-configuration annotation
// - applying default values
func normalizeTypedValue(tv *typed.TypedValue) ([]byte, error) {
	ru := tv.AsValue().Unstructured()
	r, ok := ru.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("error converting result typedValue: expected map got %T", ru)
	}
	resultUn := &unstructured.Unstructured{Object: r}
	unstructured.RemoveNestedField(resultUn.Object, "metadata", "annotations", AnnotationLastAppliedConfig)

	resultBytes, err := json.Marshal(resultUn)
	if err != nil {
		return nil, fmt.Errorf("error while marshaling merged unstructured: %w", err)
	}

	obj, err := scheme.Scheme.New(resultUn.GroupVersionKind())
	if err == nil {
		err := json.Unmarshal(resultBytes, &obj)
		if err != nil {
			return nil, fmt.Errorf("error unmarshaling merged bytes into object: %w", err)
		}
		resultBytes, err = patchDefaultValues(resultBytes, obj)
		if err != nil {
			return nil, fmt.Errorf("error applying defaults: %w", err)
		}
	}
	return resultBytes, nil
}

func buildDiffResult(predictedBytes []byte, liveBytes []byte) *DiffResult {
	return &DiffResult{
		Modified:       string(liveBytes) != string(predictedBytes),
		NormalizedLive: liveBytes,
		PredictedLive:  predictedBytes,
	}
}

// TwoWayDiff performs a three-way diff and uses specified config as a recently applied config
func TwoWayDiff(config, live *unstructured.Unstructured) (*DiffResult, error) {
	if live != nil && config != nil {
		return ThreeWayDiff(config, config.DeepCopy(), live)
	}
	return handleResourceCreateOrDeleteDiff(config, live)
}

// handleResourceCreateOrDeleteDiff will calculate the diff in case of resource creation or
// deletion. Expects that config or live is nil which means that the resource is being
// created or being deleted. Will return error if both are nil or if none are nil.
func handleResourceCreateOrDeleteDiff(config, live *unstructured.Unstructured) (*DiffResult, error) {
	if live != nil && config != nil {
		return nil, errors.New("unnexpected state: expected live or config to be null: not create or delete operation")
	}
	if live != nil {
		liveData, err := json.Marshal(live)
		if err != nil {
			return nil, err
		}
		return &DiffResult{Modified: false, NormalizedLive: liveData, PredictedLive: []byte("null")}, nil
	} else if config != nil {
		predictedLiveData, err := json.Marshal(config.Object)
		if err != nil {
			return nil, err
		}
		return &DiffResult{Modified: true, NormalizedLive: []byte("null"), PredictedLive: predictedLiveData}, nil
	}
	return nil, errors.New("both live and config are null objects")
}

// generateSchemeDefaultPatch runs the scheme default functions on the given parameter, and
// return a patch representing the delta vs the origin parameter object.
func generateSchemeDefaultPatch(kubeObj runtime.Object) ([]byte, error) {
	// 1) Call scheme defaulter functions on a clone of our k8s resource object
	patched := kubeObj.DeepCopyObject()
	gescheme.Scheme.Default(patched)

	// 2) Compare the original object (pre-defaulter funcs) with patched object (post-default funcs),
	// and generate a patch that can be applied against the original
	patch, success, err := CreateTwoWayMergePatch(kubeObj, patched, kubeObj.DeepCopyObject())

	// Ignore empty patch: this only means that kubescheme.Scheme.Default(...) made no changes.
	if string(patch) == "{}" && err == nil {
		success = true
	}
	if err != nil || !success {
		if err == nil && !success {
			err = errors.New("empty result")
		}
		return nil, err
	}

	return patch, err
}

// applyPatch executes kubernetes server side patch:
// uses corresponding data structure, applies appropriate defaults and executes strategic merge patch
func applyPatch(liveBytes []byte, patchBytes []byte, newVersionedObject func() (runtime.Object, error)) ([]byte, []byte, error) {
	// Construct an empty instance of the object we are applying a patch against
	predictedLive, err := newVersionedObject()
	if err != nil {
		return nil, nil, err
	}

	// Apply the patchBytes patch against liveBytes, using predictedLive to indicate the k8s data type
	predictedLiveBytes, err := strategicpatch.StrategicMergePatch(liveBytes, patchBytes, predictedLive)
	if err != nil {
		return nil, nil, err
	}

	// Unmarshal predictedLiveBytes into predictedLive; note that this will discard JSON fields in predictedLiveBytes
	// which are not in the predictedLive struct. predictedLive is thus "tainted" and we should not use it directly.
	if err = json.Unmarshal(predictedLiveBytes, &predictedLive); err == nil {
		// 1) Calls 'kubescheme.Scheme.Default(predictedLive)' and generates a patch containing the delta of that
		// call, which can then be applied to predictedLiveBytes.
		//
		// Why do we do this? Since predictedLive is "tainted" (missing extra fields), we cannot use it to populate
		// predictedLiveBytes, BUT we still need predictedLive itself in order to call the default scheme functions.
		// So, we call the default scheme functions on the "tainted" struct, to generate a patch, and then
		// apply that patch to the untainted JSON.
		patch, err := generateSchemeDefaultPatch(predictedLive)
		if err != nil {
			return nil, nil, err
		}

		// 2) Apply the default-funcs patch against the original "untainted" JSON
		// This allows us to apply the scheme default values generated above, against JSON that does not fully conform
		// to its k8s resource type (eg the JSON may contain those invalid fields that we do not wish to discard).
		predictedLiveBytes, err = strategicpatch.StrategicMergePatch(predictedLiveBytes, patch, predictedLive.DeepCopyObject())
		if err != nil {
			return nil, nil, err
		}

		// 3) Unmarshall into a map[string]any, then back into byte[], to ensure the fields
		// are sorted in a consistent order (we do the same below, so that they can be
		// lexicographically compared with one another)
		var result map[string]any
		err = json.Unmarshal([]byte(predictedLiveBytes), &result)
		if err != nil {
			return nil, nil, err
		}
		predictedLiveBytes, err = json.Marshal(result)
		if err != nil {
			return nil, nil, err
		}
	}

	live, err := newVersionedObject()
	if err != nil {
		return nil, nil, err
	}

	// As above, unknown JSON fields in liveBytes will be discarded in the unmarshalling to 'live'.
	// However, this is much less likely since liveBytes is coming from a live k8s instance which
	// has already accepted those resources. Regardless, we still treat 'live' as tainted.
	if err = json.Unmarshal(liveBytes, live); err == nil {
		// As above, indirectly apply the schema defaults against liveBytes
		patch, err := generateSchemeDefaultPatch(live)
		if err != nil {
			return nil, nil, err
		}
		liveBytes, err = strategicpatch.StrategicMergePatch(liveBytes, patch, live.DeepCopyObject())
		if err != nil {
			return nil, nil, err
		}

		// Ensure the fields are sorted in a consistent order (as above)
		var result map[string]any
		err = json.Unmarshal([]byte(liveBytes), &result)
		if err != nil {
			return nil, nil, err
		}
		liveBytes, err = json.Marshal(result)
		if err != nil {
			return nil, nil, err
		}
	}

	return liveBytes, predictedLiveBytes, nil
}

// patchDefaultValues will calculate the default values patch based on the
// given obj. It will apply the patch using the given objBytes and return
// the new patched object.
func patchDefaultValues(objBytes []byte, obj runtime.Object) ([]byte, error) {
	// 1) Call 'kubescheme.Scheme.Default(obj)' to generate a patch containing
	// the default values for the given scheme.
	patch, err := generateSchemeDefaultPatch(obj)
	if err != nil {
		return nil, fmt.Errorf("error generating patch for default values: %w", err)
	}

	// 2) Apply the patch with default values in objBytes.
	patchedBytes, err := strategicpatch.StrategicMergePatch(objBytes, patch, obj)
	if err != nil {
		return nil, fmt.Errorf("error applying patch for default values: %w", err)
	}

	// 3) Unmarshall into a map[string]any, then back into byte[], to
	// ensure the fields are sorted in a consistent order (we do the same below,
	// so that they can be lexicographically compared with one another).
	var result map[string]any
	err = json.Unmarshal([]byte(patchedBytes), &result)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling patched bytes: %w", err)
	}
	patchedBytes, err = json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("error marshaling patched bytes: %w", err)
	}

	return patchedBytes, nil
}

// ThreeWayDiff performs a diff with the understanding of how to incorporate the
// last-applied-configuration annotation in the diff.
// Inputs are assumed to be stripped of type information
func ThreeWayDiff(orig, config, live *unstructured.Unstructured) (*DiffResult, error) {
	orig = removeNamespaceAnnotation(orig)
	config = removeNamespaceAnnotation(config)

	// 1. calculate a 3-way merge patch
	patchBytes, newVersionedObject, err := threeWayMergePatch(orig, config, live)
	if err != nil {
		return nil, err
	}

	// 2. get expected live object by applying the patch against the live object
	liveBytes, err := json.Marshal(live)
	if err != nil {
		return nil, err
	}

	var predictedLiveBytes []byte
	// If orig/config/live represents a registered scheme...
	if newVersionedObject != nil {
		// Apply patch while applying scheme defaults
		liveBytes, predictedLiveBytes, err = applyPatch(liveBytes, patchBytes, newVersionedObject)
		if err != nil {
			return nil, err
		}
	} else {
		// Otherwise, merge patch directly as JSON
		predictedLiveBytes, err = jsonpatch.MergePatch(liveBytes, patchBytes)
		if err != nil {
			return nil, err
		}
	}

	return buildDiffResult(predictedLiveBytes, liveBytes), nil
}

// removeNamespaceAnnotation remove the namespace and an empty annotation map from the metadata.
// The namespace field is present in live (namespaced) objects, but not necessarily present in
// config or last-applied. This results in a diff which we don't care about. We delete the two so
// that the diff is more relevant.
func removeNamespaceAnnotation(orig *unstructured.Unstructured) *unstructured.Unstructured {
	orig = orig.DeepCopy()
	if metadataIf, ok := orig.Object["metadata"]; ok {
		metadata := metadataIf.(map[string]any)
		delete(metadata, "namespace")
		if annotationsIf, ok := metadata["annotations"]; ok {
			shouldDelete := false
			if annotationsIf == nil {
				shouldDelete = true
			} else {
				annotation, ok := annotationsIf.(map[string]any)
				if ok && len(annotation) == 0 {
					shouldDelete = true
				}
			}
			if shouldDelete {
				delete(metadata, "annotations")
			}
		}
	}
	return orig
}

// StatefulSet requires special handling since it embeds PersistentVolumeClaim resource.
// K8S API server applies additional default field which we cannot reproduce on client side.
// So workaround is to remove all "defaulted" fields from 'volumeClaimTemplates' of live resource.
func statefulSetWorkaround(orig, live *unstructured.Unstructured) *unstructured.Unstructured {
	origTemplate, ok, err := unstructured.NestedSlice(orig.Object, "spec", "volumeClaimTemplates")
	if !ok || err != nil {
		return live
	}

	liveTemplate, ok, err := unstructured.NestedSlice(live.Object, "spec", "volumeClaimTemplates")
	if !ok || err != nil {
		return live
	}
	live = live.DeepCopy()

	_ = unstructured.SetNestedField(live.Object, jsonutil.RemoveListFields(origTemplate, liveTemplate), "spec", "volumeClaimTemplates")
	return live
}

func threeWayMergePatch(orig, config, live *unstructured.Unstructured) ([]byte, func() (runtime.Object, error), error) {
	origBytes, err := json.Marshal(orig.Object)
	if err != nil {
		return nil, nil, err
	}
	configBytes, err := json.Marshal(config.Object)
	if err != nil {
		return nil, nil, err
	}

	if versionedObject, err := scheme.Scheme.New(orig.GroupVersionKind()); err == nil {
		gk := orig.GroupVersionKind().GroupKind()
		if (gk.Group == "apps" || gk.Group == "extensions") && gk.Kind == "StatefulSet" {
			live = statefulSetWorkaround(orig, live)
		}

		liveBytes, err := json.Marshal(live.Object)
		if err != nil {
			return nil, nil, err
		}

		lookupPatchMeta, err := strategicpatch.NewPatchMetaFromStruct(versionedObject)
		if err != nil {
			return nil, nil, err
		}
		patch, err := strategicpatch.CreateThreeWayMergePatch(origBytes, configBytes, liveBytes, lookupPatchMeta, true)
		if err != nil {
			return nil, nil, err
		}
		newVersionedObject := func() (runtime.Object, error) {
			return scheme.Scheme.New(orig.GroupVersionKind())
		}
		return patch, newVersionedObject, nil
	}
	// Remove defaulted fields from the live object.
	// This subtracts any extra fields in the live object which are not present in last-applied-configuration.
	live = &unstructured.Unstructured{Object: jsonutil.RemoveMapFields(orig.Object, live.Object)}

	liveBytes, err := json.Marshal(live.Object)
	if err != nil {
		return nil, nil, err
	}

	patch, err := jsonmergepatch.CreateThreeWayJSONMergePatch(origBytes, configBytes, liveBytes)
	if err != nil {
		return nil, nil, err
	}
	return patch, nil, nil
}

func GetLastAppliedConfigAnnotation(live *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if live == nil {
		return nil, nil
	}
	annotations := live.GetAnnotations()
	lastAppliedStr, ok := annotations[corev1.LastAppliedConfigAnnotation]
	if !ok {
		return nil, nil
	}
	var obj unstructured.Unstructured
	err := json.Unmarshal([]byte(lastAppliedStr), &obj)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s in %s: %w", corev1.LastAppliedConfigAnnotation, live.GetName(), err)
	}
	return &obj, nil
}

// DiffArray performs a diff on a list of unstructured objects. Objects are expected to match
// environments
func DiffArray(configArray, liveArray []*unstructured.Unstructured, opts ...Option) (*DiffResultList, error) {
	numItems := len(configArray)
	if len(liveArray) != numItems {
		return nil, errors.New("left and right arrays have mismatched lengths")
	}

	diffResultList := DiffResultList{
		Diffs: make([]DiffResult, numItems),
	}
	for i := 0; i < numItems; i++ {
		config := configArray[i]
		live := liveArray[i]
		diffRes, err := Diff(config, live, opts...)
		if err != nil {
			return nil, err
		}
		diffResultList.Diffs[i] = *diffRes
		if diffRes.Modified {
			diffResultList.Modified = true
		}
	}
	return &diffResultList, nil
}

func Normalize(un *unstructured.Unstructured, opts ...Option) {
	if un == nil {
		return
	}
	o := applyOptions(opts)

	// creationTimestamp is sometimes set to null in the config when exported (e.g. SealedSecrets)
	// Removing the field allows a cleaner diff.
	unstructured.RemoveNestedField(un.Object, "metadata", "creationTimestamp")

	gvk := un.GroupVersionKind()
	switch {
	case gvk.Group == "" && gvk.Kind == "Secret":
		NormalizeSecret(un, opts...)
	case gvk.Group == "rbac.authorization.k8s.io" && (gvk.Kind == "ClusterRole" || gvk.Kind == "Role"):
		normalizeRole(un, o)
	case gvk.Group == "" && gvk.Kind == "Endpoints":
		normalizeEndpoint(un, o)
	}

	err := o.normalizer.Normalize(un)
	if err != nil {
		o.log.Error(err, fmt.Sprintf("Failed to normalize %s/%s/%s", un.GroupVersionKind(), un.GetNamespace(), un.GetName()))
	}
}

// NormalizeSecret mutates the supplied object and encodes stringData to data, and converts nils to
// empty strings. If the object is not a secret, or is an invalid secret, then returns the same object.
func NormalizeSecret(un *unstructured.Unstructured, opts ...Option) {
	if un == nil {
		return
	}
	gvk := un.GroupVersionKind()
	if gvk.Group != "" || gvk.Kind != "Secret" {
		return
	}

	// move stringData to data section
	if stringData, found, err := unstructured.NestedMap(un.Object, "stringData"); found && err == nil {
		var data map[string]any
		data, found, _ = unstructured.NestedMap(un.Object, "data")
		if !found {
			data = make(map[string]any)
		}

		// base64 encode string values and add non-string values as is.
		// This ensures that the apply fails if the secret is invalid.
		for k, v := range stringData {
			strVal, ok := v.(string)
			if ok {
				data[k] = base64.StdEncoding.EncodeToString([]byte(strVal))
			} else {
				data[k] = v
			}
		}

		err := unstructured.SetNestedField(un.Object, data, "data")
		if err == nil {
			delete(un.Object, "stringData")
		}
	}

	o := applyOptions(opts)
	var secret corev1.Secret
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(un.Object, &secret)
	if err != nil {
		o.log.Error(err, "Failed to convert from unstructured into Secret")
		return
	}
	// We normalize nils to empty string to handle: https://github.com/argoproj/argo-cd/issues/943
	for k, v := range secret.Data {
		if len(v) == 0 {
			secret.Data[k] = []byte("")
		}
	}
	newObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&secret)
	if err != nil {
		o.log.Error(err, "object unable to convert from secret")
		return
	}
	if secret.Data != nil {
		err = unstructured.SetNestedField(un.Object, newObj["data"], "data")
		if err != nil {
			o.log.Error(err, "failed to set secret.data")
			return
		}
	}
}

// normalizeEndpoint normalizes endpoint meaning that EndpointSubsets are sorted lexicographically
func normalizeEndpoint(un *unstructured.Unstructured, o options) {
	if un == nil {
		return
	}
	gvk := un.GroupVersionKind()
	if gvk.Group != "" || gvk.Kind != "Endpoints" {
		return
	}
	var ep corev1.Endpoints
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(un.Object, &ep)
	if err != nil {
		o.log.Error(err, "Failed to convert from unstructured into Endpoints")
		return
	}

	// add default protocol to subsets ports if it is empty
	for s := range ep.Subsets {
		subset := &ep.Subsets[s]
		for p := range subset.Ports {
			port := &subset.Ports[p]
			if port.Protocol == "" {
				port.Protocol = corev1.ProtocolTCP
			}
		}
	}

	endpoints.SortSubsets(ep.Subsets)

	newObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&ep)
	if err != nil {
		o.log.Info(fmt.Sprintf(couldNotMarshalErrMsg, gvk, err))
		return
	}
	un.Object = newObj
}

// normalizeRole mutates the supplied Role/ClusterRole and sets rules to null if it is an empty list or an aggregated role
func normalizeRole(un *unstructured.Unstructured, o options) {
	if un == nil {
		return
	}
	gvk := un.GroupVersionKind()
	if gvk.Group != "rbac.authorization.k8s.io" || (gvk.Kind != "Role" && gvk.Kind != "ClusterRole") {
		return
	}

	// Check whether the role we're checking is an aggregation role. If it is, we ignore any differences in rules.
	if o.ignoreAggregatedRoles {
		aggrIf, ok := un.Object["aggregationRule"]
		if ok {
			_, ok = aggrIf.(map[string]any)
			if !ok {
				o.log.Info(fmt.Sprintf("Malformed aggregationRule in resource '%s', won't modify.", un.GetName()))
			} else {
				un.Object["rules"] = nil
			}
		}
	}

	rulesIf, ok := un.Object["rules"]
	if !ok {
		return
	}
	rules, ok := rulesIf.([]any)
	if !ok {
		return
	}
	if rules != nil && len(rules) == 0 {
		un.Object["rules"] = nil
	}
}

// CreateTwoWayMergePatch is a helper to construct a two-way merge patch from objects (instead of bytes)
func CreateTwoWayMergePatch(orig, new, dataStruct any) ([]byte, bool, error) {
	origBytes, err := json.Marshal(orig)
	if err != nil {
		return nil, false, err
	}
	newBytes, err := json.Marshal(new)
	if err != nil {
		return nil, false, err
	}
	patch, err := strategicpatch.CreateTwoWayMergePatch(origBytes, newBytes, dataStruct)
	if err != nil {
		return nil, false, err
	}
	return patch, string(patch) != "{}", nil
}

// HideSecretData replaces secret data & optional annotations values in specified target, live secrets and in last applied configuration of live secret with plus(+). Also preserves differences between
// target, live and last applied config values. E.g. if all three are equal the values would be replaced with same number of plus(+). If all are different then number of plus(+)
// in replacement should be different.
func HideSecretData(target *unstructured.Unstructured, live *unstructured.Unstructured, hideAnnotations map[string]bool) (*unstructured.Unstructured, *unstructured.Unstructured, error) {
	var liveLastAppliedAnnotation *unstructured.Unstructured
	if live != nil {
		liveLastAppliedAnnotation, _ = GetLastAppliedConfigAnnotation(live)
		live = live.DeepCopy()
	}
	if target != nil {
		target = target.DeepCopy()
	}

	keys := map[string]bool{}
	for _, obj := range []*unstructured.Unstructured{target, live, liveLastAppliedAnnotation} {
		if obj == nil {
			continue
		}
		NormalizeSecret(obj)
		if data, found, err := unstructured.NestedMap(obj.Object, "data"); found && err == nil {
			for k := range data {
				keys[k] = true
			}
		}
	}

	var err error
	target, live, liveLastAppliedAnnotation, err = hide(target, live, liveLastAppliedAnnotation, keys, "data")
	if err != nil {
		return nil, nil, err
	}

	target, live, liveLastAppliedAnnotation, err = hide(target, live, liveLastAppliedAnnotation, hideAnnotations, "metadata", "annotations")
	if err != nil {
		return nil, nil, err
	}

	if live != nil && liveLastAppliedAnnotation != nil {
		annotations := live.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		// special case: hide "kubectl.kubernetes.io/last-applied-configuration" annotation
		if _, ok := hideAnnotations[corev1.LastAppliedConfigAnnotation]; ok {
			annotations[corev1.LastAppliedConfigAnnotation] = replacement
		} else {
			lastAppliedData, err := json.Marshal(liveLastAppliedAnnotation)
			if err != nil {
				return nil, nil, fmt.Errorf("error marshaling json: %w", err)
			}
			annotations[corev1.LastAppliedConfigAnnotation] = string(lastAppliedData)
		}
		live.SetAnnotations(annotations)
	}
	return target, live, nil
}

func hide(target, live, liveLastAppliedAnnotation *unstructured.Unstructured, keys map[string]bool, fields ...string) (*unstructured.Unstructured, *unstructured.Unstructured, *unstructured.Unstructured, error) {
	for k := range keys {
		// we use "+" rather than the more common "*"
		nextReplacement := replacement
		valToReplacement := make(map[string]string)
		for _, obj := range []*unstructured.Unstructured{target, live, liveLastAppliedAnnotation} {
			var data map[string]any
			if obj != nil {
				// handles an edge case when secret data has nil value
				// https://github.com/argoproj/argo-cd/issues/5584
				dataValue, ok, _ := unstructured.NestedFieldCopy(obj.Object, fields...)
				if ok {
					if dataValue == nil {
						continue
					}
				}
				var err error
				data, _, err = unstructured.NestedMap(obj.Object, fields...)
				if err != nil {
					return nil, nil, nil, fmt.Errorf("unstructured.NestedMap error: %w", err)
				}
			}
			if data == nil {
				data = make(map[string]any)
			}
			valData, ok := data[k]
			if !ok {
				continue
			}
			val := toString(valData)
			replacement, ok := valToReplacement[val]
			if !ok {
				replacement = nextReplacement
				nextReplacement = nextReplacement + "++++"
				valToReplacement[val] = replacement
			}
			data[k] = replacement
			err := unstructured.SetNestedField(obj.Object, data, fields...)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("unstructured.SetNestedField error: %w", err)
			}
		}
	}
	return target, live, liveLastAppliedAnnotation, nil
}

func toString(val any) string {
	if val == nil {
		return ""
	}
	return fmt.Sprintf("%s", val)
}

// remarshal checks resource kind and version and re-marshal using corresponding struct custom marshaller.
// This ensures that expected resource state is formatter same as actual resource state in kubernetes
// and allows to find differences between actual and target states more accurately.
// Remarshalling also strips any type information (e.g. float64 vs. int) from the unstructured
// object. This is important for diffing since it will cause godiff to report a false difference.
func remarshal(obj *unstructured.Unstructured, o options) *unstructured.Unstructured {
	data, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}

	// Unmarshal again to strip type information (e.g. float64 vs. int) from the unstructured
	// object. This is important for diffing since it will cause godiff to report a false difference.
	var newUn unstructured.Unstructured
	err = json.Unmarshal(data, &newUn)
	if err != nil {
		panic(err)
	}
	obj = &newUn

	gvk := obj.GroupVersionKind()
	item, err := scheme.Scheme.New(obj.GroupVersionKind())
	if err != nil {
		// This is common. the scheme is not registered
		o.log.V(1).Info(fmt.Sprintf("Could not create new object of type %s: %v", gvk, err))
		return obj
	}
	// This will drop any omitempty fields, perform resource conversion etc...
	unmarshalledObj := reflect.New(reflect.TypeOf(item).Elem()).Interface()
	// Unmarshal data into unmarshalledObj, but detect if there are any unknown fields that are not
	// found in the target GVK object.
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&unmarshalledObj); err != nil {
		// Likely a field present in obj that is not present in the GVK type, or user
		// may have specified an invalid spec in git, so return original object
		o.log.V(1).Info(fmt.Sprintf(couldNotMarshalErrMsg, gvk, err))
		return obj
	}
	unstrBody, err := runtime.DefaultUnstructuredConverter.ToUnstructured(unmarshalledObj)
	if err != nil {
		o.log.V(1).Info(fmt.Sprintf(couldNotMarshalErrMsg, gvk, err))
		return obj
	}
	// Remove all default values specified by custom formatter (e.g. creationTimestamp)
	unstrBody = jsonutil.RemoveMapFields(obj.Object, unstrBody)
	return &unstructured.Unstructured{Object: unstrBody}
}
