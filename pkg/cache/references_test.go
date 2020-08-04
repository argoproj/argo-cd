package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
)

func mustToUnstructured(obj interface{}) *unstructured.Unstructured {
	un, err := kube.ToUnstructured(obj)
	if err != nil {
		panic(err)
	}
	return un
}

func TestGetPVCOwnerRefs(t *testing.T) {
	stsRef := []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: kube.StatefulSetKind, Name: "web", UID: "123"}}
	cluster := newCluster()
	replicas := int32(1)
	sts := mustToUnstructured(&appsv1.StatefulSet{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: kube.StatefulSetKind},
		ObjectMeta: metav1.ObjectMeta{UID: "123", Name: "web", Namespace: "default"},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			VolumeClaimTemplates: []v1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "www",
				},
			}},
		},
	})
	cluster.resources[kube.GetResourceKey(sts)] = cluster.newResource(sts)

	t.Run("STSNameNotMatching", func(t *testing.T) {
		refs := cluster.getPVCOwnerRefs(mustToUnstructured(&v1.PersistentVolumeClaim{
			TypeMeta:   metav1.TypeMeta{Kind: kube.PersistentVolumeClaimKind},
			ObjectMeta: metav1.ObjectMeta{Name: "www-web1-0", Namespace: "default"},
		}))

		assert.Len(t, refs, 0)
	})

	t.Run("STSTemplateNameNotMatching", func(t *testing.T) {
		refs := cluster.getPVCOwnerRefs(mustToUnstructured(&v1.PersistentVolumeClaim{
			TypeMeta:   metav1.TypeMeta{Kind: kube.PersistentVolumeClaimKind},
			ObjectMeta: metav1.ObjectMeta{Name: "www1-web-0", Namespace: "default"},
		}))

		assert.Len(t, refs, 0)
	})

	t.Run("MatchingSTSExists", func(t *testing.T) {
		refs := cluster.getPVCOwnerRefs(mustToUnstructured(&v1.PersistentVolumeClaim{
			TypeMeta:   metav1.TypeMeta{Kind: kube.PersistentVolumeClaimKind},
			ObjectMeta: metav1.ObjectMeta{Name: "www-web-0", Namespace: "default"},
		}))

		assert.ElementsMatch(t, refs, stsRef)
	})

}
