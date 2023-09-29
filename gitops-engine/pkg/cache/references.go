package cache

import (
	"encoding/json"
	"fmt"
	"strings"

	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
)

// mightHaveInferredOwner returns true of given resource might have inferred owners
func mightHaveInferredOwner(r *Resource) bool {
	return r.Ref.GroupVersionKind().Group == "" && r.Ref.Kind == kube.PersistentVolumeClaimKind
}

func (c *clusterCache) resolveResourceReferences(un *unstructured.Unstructured) ([]metav1.OwnerReference, func(kube.ResourceKey) bool) {
	var isInferredParentOf func(_ kube.ResourceKey) bool
	ownerRefs := un.GetOwnerReferences()
	gvk := un.GroupVersionKind()

	switch {

	// Special case for endpoint. Remove after https://github.com/kubernetes/kubernetes/issues/28483 is fixed
	case gvk.Group == "" && gvk.Kind == kube.EndpointsKind && len(un.GetOwnerReferences()) == 0:
		ownerRefs = append(ownerRefs, metav1.OwnerReference{
			Name:       un.GetName(),
			Kind:       kube.ServiceKind,
			APIVersion: "v1",
		})

	// Special case for Operator Lifecycle Manager ClusterServiceVersion:
	case un.GroupVersionKind().Group == "operators.coreos.com" && un.GetKind() == "ClusterServiceVersion":
		if un.GetAnnotations()["olm.operatorGroup"] != "" {
			ownerRefs = append(ownerRefs, metav1.OwnerReference{
				Name:       un.GetAnnotations()["olm.operatorGroup"],
				Kind:       "OperatorGroup",
				APIVersion: "operators.coreos.com/v1",
			})
		}

	// Edge case: consider auto-created service account tokens as a child of service account objects
	case un.GetKind() == kube.SecretKind && un.GroupVersionKind().Group == "":
		if yes, ref := isServiceAccountTokenSecret(un); yes {
			ownerRefs = append(ownerRefs, ref)
		}

	case (un.GroupVersionKind().Group == "apps" || un.GroupVersionKind().Group == "extensions") && un.GetKind() == kube.StatefulSetKind:
		if refs, err := isStatefulSetChild(un); err != nil {
			c.log.Error(err, fmt.Sprintf("Failed to extract StatefulSet %s/%s PVC references", un.GetNamespace(), un.GetName()))
		} else {
			isInferredParentOf = refs
		}
	}

	return ownerRefs, isInferredParentOf
}

func isStatefulSetChild(un *unstructured.Unstructured) (func(kube.ResourceKey) bool, error) {
	sts := v1.StatefulSet{}
	data, err := json.Marshal(un)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &sts)
	if err != nil {
		return nil, err
	}

	templates := sts.Spec.VolumeClaimTemplates
	return func(key kube.ResourceKey) bool {
		if key.Kind == kube.PersistentVolumeClaimKind && key.GroupKind().Group == "" {
			for _, templ := range templates {
				if strings.HasPrefix(key.Name, fmt.Sprintf("%s-%s-", templ.Name, un.GetName())) {
					return true
				}
			}
		}
		return false
	}, nil
}

func isServiceAccountTokenSecret(un *unstructured.Unstructured) (bool, metav1.OwnerReference) {
	ref := metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       kube.ServiceAccountKind,
	}

	if typeVal, ok, err := unstructured.NestedString(un.Object, "type"); !ok || err != nil || typeVal != "kubernetes.io/service-account-token" {
		return false, ref
	}

	annotations := un.GetAnnotations()
	if annotations == nil {
		return false, ref
	}

	id, okId := annotations["kubernetes.io/service-account.uid"]
	name, okName := annotations["kubernetes.io/service-account.name"]
	if okId && okName {
		ref.Name = name
		ref.UID = types.UID(id)
	}
	return ref.Name != "" && ref.UID != "", ref
}
