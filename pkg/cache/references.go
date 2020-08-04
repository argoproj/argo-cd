package cache

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
)

var (
	pvcNameRegex = regexp.MustCompile(`.+-(.+)-\d+`)
)

func (c *clusterCache) resolveResourceReferences(un *unstructured.Unstructured) ([]metav1.OwnerReference, func(kube.ResourceKey) bool) {
	var isChildRef func(_ kube.ResourceKey) bool
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

	// PVC with matching names should be considered as a child of matching StatefulSet
	case un.GroupVersionKind().Group == "" && un.GroupVersionKind().Kind == kube.PersistentVolumeClaimKind:
		ownerRefs = append(ownerRefs, c.getPVCOwnerRefs(un)...)

	case (un.GroupVersionKind().Group == "apps" || un.GroupVersionKind().Group == "extensions") && un.GetKind() == kube.StatefulSetKind:
		if refs, err := isStatefulSetChild(un); err != nil {
			log.Errorf("Failed to extract StatefulSet %s/%s PVC references: %v", un.GetNamespace(), un.GetName(), err)
		} else {
			isChildRef = refs
		}
	}

	return ownerRefs, isChildRef
}

func (c *clusterCache) getPVCOwnerRefs(un *unstructured.Unstructured) []metav1.OwnerReference {
	if match := pvcNameRegex.FindStringSubmatch(un.GetName()); len(match) >= 2 {
		stsName := match[1]
		if sts, ok := c.resources[kube.NewResourceKey("apps", kube.StatefulSetKind, un.GetNamespace(), stsName)]; ok &&
			sts.isChildRef != nil && sts.isChildRef(kube.GetResourceKey(un)) {

			return []metav1.OwnerReference{{
				APIVersion: sts.Ref.APIVersion,
				Kind:       sts.Ref.Kind,
				Name:       sts.Ref.Name,
				UID:        sts.Ref.UID,
			}}
		}
	}
	return nil
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
		for _, templ := range templates {
			if strings.HasPrefix(key.Name, fmt.Sprintf("%s-%s-", templ.Name, un.GetName())) {
				return true
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
