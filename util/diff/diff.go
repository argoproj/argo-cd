package diff

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"reflect"

	"github.com/ghodss/yaml"
	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"
	"github.com/yudai/gojsondiff"
	"github.com/yudai/gojsondiff/formatter"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	jsonutil "github.com/argoproj/argo-cd/util/json"
)

type DiffResult struct {
	Diff     gojsondiff.Diff
	Modified bool
}

type DiffResultList struct {
	Diffs    []DiffResult
	Modified bool
}

type Normalizer interface {
	Normalize(un *unstructured.Unstructured) error
}

// Diff performs a diff on two unstructured objects. If the live object happens to have a
// "kubectl.kubernetes.io/last-applied-configuration", then perform a three way diff.
func Diff(config, live *unstructured.Unstructured, normalizer Normalizer) *DiffResult {
	if config != nil {
		config = remarshal(config)
		Normalize(config, normalizer)
	}
	if live != nil {
		live = remarshal(live)
		Normalize(live, normalizer)
	}
	orig := GetLastAppliedConfigAnnotation(live)
	if orig != nil && config != nil {
		Normalize(orig, normalizer)
		dr, err := ThreeWayDiff(orig, config, live)
		if err == nil {
			return dr
		}
		log.Debugf("three-way diff calculation failed: %v. Falling back to two-way diff", err)
	}
	return TwoWayDiff(config, live)
}

// TwoWayDiff performs a normal two-way diff between two unstructured objects. Ignores extra fields
// in the live object.
// Inputs are assumed to be stripped of type information
func TwoWayDiff(config, live *unstructured.Unstructured) *DiffResult {
	var configObj, liveObj map[string]interface{}
	if config != nil {
		config = removeNamespaceAnnotation(config)
		configObj = config.Object
	}
	if live != nil {
		liveObj = jsonutil.RemoveMapFields(configObj, live.Object)
	}
	gjDiff := gojsondiff.New().CompareObjects(liveObj, configObj)
	dr := DiffResult{
		Diff:     gjDiff,
		Modified: gjDiff.Modified(),
	}
	return &dr
}

// ThreeWayDiff performs a diff with the understanding of how to incorporate the
// last-applied-configuration annotation in the diff.
// Inputs are assumed to be stripped of type information
func ThreeWayDiff(orig, config, live *unstructured.Unstructured) (*DiffResult, error) {
	orig = removeNamespaceAnnotation(orig)
	config = removeNamespaceAnnotation(config)
	// Remove defaulted fields from the live object.
	// This subtracts any extra fields in the live object which are not present in last-applied-configuration.
	// This is needed to perform a fair comparison when we send the objects to gojsondiff
	live = &unstructured.Unstructured{Object: jsonutil.RemoveMapFields(orig.Object, live.Object)}

	// 1. calculate a 3-way merge patch
	patchBytes, err := threeWayMergePatch(orig, config, live)
	if err != nil {
		return nil, err
	}

	// 2. apply the patch against the live object
	liveBytes, err := json.Marshal(live)
	if err != nil {
		return nil, err
	}
	versionedObject, err := scheme.Scheme.New(orig.GroupVersionKind())
	if err != nil {
		return nil, err
	}
	patchedLiveBytes, err := strategicpatch.StrategicMergePatch(liveBytes, patchBytes, versionedObject)
	if err != nil {
		return nil, err
	}
	var patchedLive unstructured.Unstructured
	err = json.Unmarshal(patchedLiveBytes, &patchedLive)
	if err != nil {
		return nil, err
	}

	// 3. diff the live object vs. the patched live object
	gjDiff := gojsondiff.New().CompareObjects(live.Object, patchedLive.Object)
	dr := DiffResult{
		Diff:     gjDiff,
		Modified: gjDiff.Modified(),
	}
	return &dr, nil
}

// stripTypeInformation strips any type information (e.g. float64 vs. int) from the unstructured
// object by remarshalling the object. This is important for diffing since it will cause godiff
// to report a false difference.
func stripTypeInformation(un *unstructured.Unstructured) *unstructured.Unstructured {
	unBytes, err := json.Marshal(un)
	if err != nil {
		panic(err)
	}
	var newUn unstructured.Unstructured
	err = json.Unmarshal(unBytes, &newUn)
	if err != nil {
		panic(err)
	}
	return &newUn
}

// removeNamespaceAnnotation remove the namespace and an empty annotation map from the metadata.
// The namespace field is present in live (namespaced) objects, but not necessarily present in
// config or last-applied. This results in a diff which we don't care about. We delete the two so
// that the diff is more relevant.
func removeNamespaceAnnotation(orig *unstructured.Unstructured) *unstructured.Unstructured {
	orig = orig.DeepCopy()
	if metadataIf, ok := orig.Object["metadata"]; ok {
		metadata := metadataIf.(map[string]interface{})
		delete(metadata, "namespace")
		if annotationsIf, ok := metadata["annotations"]; ok {
			shouldDelete := false
			if annotationsIf == nil {
				shouldDelete = true
			} else {
				annotation := annotationsIf.(map[string]interface{})
				if len(annotation) == 0 {
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

func threeWayMergePatch(orig, config, live *unstructured.Unstructured) ([]byte, error) {
	origBytes, err := json.Marshal(orig.Object)
	if err != nil {
		return nil, err
	}
	configBytes, err := json.Marshal(config.Object)
	if err != nil {
		return nil, err
	}
	liveBytes, err := json.Marshal(live.Object)
	if err != nil {
		return nil, err
	}
	gvk := orig.GroupVersionKind()
	versionedObject, err := scheme.Scheme.New(gvk)
	if err != nil {
		return nil, err
	}
	lookupPatchMeta, err := strategicpatch.NewPatchMetaFromStruct(versionedObject)
	if err != nil {
		return nil, err
	}
	return strategicpatch.CreateThreeWayMergePatch(origBytes, configBytes, liveBytes, lookupPatchMeta, true)
}

func GetLastAppliedConfigAnnotation(live *unstructured.Unstructured) *unstructured.Unstructured {
	if live == nil {
		return nil
	}
	annots := live.GetAnnotations()
	if annots == nil {
		return nil
	}
	lastAppliedStr, ok := annots[corev1.LastAppliedConfigAnnotation]
	if !ok {
		return nil
	}
	var obj unstructured.Unstructured
	err := json.Unmarshal([]byte(lastAppliedStr), &obj)
	if err != nil {
		log.Warnf("Failed to unmarshal %s in %s", core.LastAppliedConfigAnnotation, live.GetName())
		return nil
	}
	return &obj
}

// DiffArray performs a diff on a list of unstructured objects. Objects are expected to match
// environments
func DiffArray(configArray, liveArray []*unstructured.Unstructured, normalizer Normalizer) (*DiffResultList, error) {
	numItems := len(configArray)
	if len(liveArray) != numItems {
		return nil, fmt.Errorf("left and right arrays have mismatched lengths")
	}

	diffResultList := DiffResultList{
		Diffs: make([]DiffResult, numItems),
	}
	for i := 0; i < numItems; i++ {
		config := configArray[i]
		live := liveArray[i]
		diffRes := Diff(config, live, normalizer)
		diffResultList.Diffs[i] = *diffRes
		if diffRes.Modified {
			diffResultList.Modified = true
		}
	}
	return &diffResultList, nil
}

// ASCIIFormat returns the ASCII format of the diff
func (d *DiffResult) ASCIIFormat(left *unstructured.Unstructured, formatOpts formatter.AsciiFormatterConfig) (string, error) {
	if !d.Diff.Modified() {
		return "", nil
	}
	if left == nil {
		return "", errors.New("Supplied nil left object")
	}
	asciiFmt := formatter.NewAsciiFormatter(left.Object, formatOpts)
	return asciiFmt.Format(d.Diff)
}

func Normalize(un *unstructured.Unstructured, normalizer Normalizer) {
	if un == nil {
		return
	}

	// creationTimestamp is sometimes set to null in the config when exported (e.g. SealedSecrets)
	// Removing the field allows a cleaner diff.
	unstructured.RemoveNestedField(un.Object, "metadata", "creationTimestamp")
	gvk := un.GroupVersionKind()
	if gvk.Group == "" && gvk.Kind == "Secret" {
		NormalizeSecret(un)
	} else if gvk.Group == "rbac.authorization.k8s.io" && (gvk.Kind == "ClusterRole" || gvk.Kind == "Role") {
		normalizeRole(un)
	}

	if normalizer != nil {
		err := normalizer.Normalize(un)
		if err != nil {
			log.Warnf("Failed to normalize %s/%s/%s: %v", un.GroupVersionKind(), un.GetNamespace(), un.GetName(), err)
		}
	}
}

// NormalizeSecret mutates the supplied object and encodes stringData to data, and converts nils to
// empty strings. If the object is not a secret, or is an invalid secret, then returns the same object.
func NormalizeSecret(un *unstructured.Unstructured) {
	if un == nil {
		return
	}
	gvk := un.GroupVersionKind()
	if gvk.Group != "" || gvk.Kind != "Secret" {
		return
	}
	var secret corev1.Secret
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(un.Object, &secret)
	if err != nil {
		return
	}
	// We normalize nils to empty string to handle: https://github.com/argoproj/argo-cd/issues/943
	for k, v := range secret.Data {
		if len(v) == 0 {
			secret.Data[k] = []byte("")
		}
	}
	if len(secret.StringData) > 0 {
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}
		for k, v := range secret.StringData {
			secret.Data[k] = []byte(v)
		}
		delete(un.Object, "stringData")
	}
	newObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&secret)
	if err != nil {
		log.Warnf("object unable to convert from secret: %v", err)
		return
	}
	if secret.Data != nil {
		err = unstructured.SetNestedField(un.Object, newObj["data"], "data")
		if err != nil {
			log.Warnf("failed to set secret.data: %v", err)
			return
		}
	}
}

// normalizeRole mutates the supplied Role/ClusterRole and sets rules to null if it is an empty list
func normalizeRole(un *unstructured.Unstructured) {
	if un == nil {
		return
	}
	gvk := un.GroupVersionKind()
	if gvk.Group != "rbac.authorization.k8s.io" || (gvk.Kind != "Role" && gvk.Kind != "ClusterRole") {
		return
	}
	rulesIf, ok := un.Object["rules"]
	if !ok {
		return
	}
	rules, ok := rulesIf.([]interface{})
	if !ok {
		return
	}
	if rules != nil && len(rules) == 0 {
		un.Object["rules"] = nil
	}
}

// JSONFormat returns the diff as a JSON string
func (d *DiffResult) JSONFormat() (string, error) {
	if !d.Diff.Modified() {
		return "", nil
	}
	jsonFmt := formatter.NewDeltaFormatter()
	return jsonFmt.Format(d.Diff)
}

// CreateTwoWayMergePatch is a helper to construct a two-way merge patch from objects (instead of bytes)
func CreateTwoWayMergePatch(orig, new, dataStruct interface{}) ([]byte, bool, error) {
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

// HideSecretData replaces secret data values in specified target, live secrets and in last applied configuration of live secret with stars. Also preserves differences between
// target, live and last applied config values. E.g. if all three are equal the values would be replaced with same number of stars. If all the are different then number of stars
// in replacement should be different.
func HideSecretData(target *unstructured.Unstructured, live *unstructured.Unstructured) (*unstructured.Unstructured, *unstructured.Unstructured, error) {
	var orig *unstructured.Unstructured
	if live != nil {
		orig = GetLastAppliedConfigAnnotation(live)
		live = live.DeepCopy()
	}
	if target != nil {
		target = target.DeepCopy()
	}

	keys := map[string]bool{}
	for _, obj := range []*unstructured.Unstructured{target, live, orig} {
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

	for k := range keys {
		// we use "+" rather than the more common "*"
		nextReplacement := "++++++++"
		valToReplacement := make(map[string]string)
		for _, obj := range []*unstructured.Unstructured{target, live, orig} {
			var data map[string]interface{}
			if obj != nil {
				var err error
				data, _, err = unstructured.NestedMap(obj.Object, "data")
				if err != nil {
					return nil, nil, err
				}
			}
			if data == nil {
				data = make(map[string]interface{})
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
			err := unstructured.SetNestedField(obj.Object, data, "data")
			if err != nil {
				return nil, nil, err
			}
		}
	}
	if live != nil && orig != nil {
		annotations := live.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		lastAppliedData, err := json.Marshal(orig)
		if err != nil {
			return nil, nil, err
		}
		annotations[core.LastAppliedConfigAnnotation] = string(lastAppliedData)
		live.SetAnnotations(annotations)
	}
	return target, live, nil
}

func toString(val interface{}) string {
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
func remarshal(obj *unstructured.Unstructured) *unstructured.Unstructured {
	obj = stripTypeInformation(obj)
	data, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	gvk := obj.GroupVersionKind()
	item, err := scheme.Scheme.New(obj.GroupVersionKind())
	if err != nil {
		// this is common. the scheme is not registered
		log.Debugf("Could not create new object of type %s: %v", gvk, err)
		return obj
	}
	// This will drop any omitempty fields, perform resource conversion etc...
	unmarshalledObj := reflect.New(reflect.TypeOf(item).Elem()).Interface()
	err = json.Unmarshal(data, &unmarshalledObj)
	if err != nil {
		// User may have specified an invalid spec in git. Return original object
		log.Warnf("Could not unmarshal to object of type %s: %v", gvk, err)
		return obj
	}
	unstrBody, err := runtime.DefaultUnstructuredConverter.ToUnstructured(unmarshalledObj)
	if err != nil {
		log.Warnf("Could not unmarshal to object of type %s: %v", gvk, err)
		return obj
	}
	// remove all default values specified by custom formatter (e.g. creationTimestamp)
	unstrBody = jsonutil.RemoveMapFields(obj.Object, unstrBody)
	return &unstructured.Unstructured{Object: unstrBody}
}

// PrintDiff prints a diff between two unstructured objects to stdout using an external diff utility
// Honors the diff utility set in the KUBECTL_EXTERNAL_DIFF environment variable
func PrintDiff(name string, live *unstructured.Unstructured, target *unstructured.Unstructured) error {
	tempDir, err := ioutil.TempDir("", "argocd-diff")
	if err != nil {
		return err
	}
	targetFile := path.Join(tempDir, name)
	targetData := []byte("")
	if target != nil {
		targetData, err = yaml.Marshal(target)
		if err != nil {
			return err
		}
	}
	err = ioutil.WriteFile(targetFile, targetData, 0644)
	if err != nil {
		return err
	}
	liveFile := path.Join(tempDir, fmt.Sprintf("%s-live.yaml", name))
	liveData := []byte("")
	if live != nil {
		liveData, err = yaml.Marshal(live)
		if err != nil {
			return err
		}
	}
	err = ioutil.WriteFile(liveFile, liveData, 0644)
	if err != nil {
		return err
	}
	cmdBinary := "diff"
	var args []string
	if envDiff := os.Getenv("KUBECTL_EXTERNAL_DIFF"); envDiff != "" {
		parts, err := shlex.Split(envDiff)
		if err != nil {
			return err
		}
		cmdBinary = parts[0]
		args = parts[1:]
	}
	cmd := exec.Command(cmdBinary, append(args, liveFile, targetFile)...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}
