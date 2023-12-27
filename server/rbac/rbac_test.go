package rbac

import (
	"context"
	rbacpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/rbac"
	application "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/assets"
	"github.com/argoproj/argo-cd/v2/util/rbac"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

const (
	fakeNamespace     = "fake-ns"
	fakeConfigMapName = "fake-cm"
)

func fakeConfigMap() *apiv1.ConfigMap {
	cm := apiv1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fakeConfigMapName,
			Namespace: fakeNamespace,
		},
		Data: make(map[string]string),
	}
	return &cm
}

func TestRBACServer(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(fakeConfigMap())
	enf := rbac.NewEnforcer(kubeclientset, fakeNamespace, fakeConfigMapName, nil)
	server := NewServer(enf, kubeclientset, fakeNamespace, fakeConfigMapName)
	ctx := context.Background()
	policyRule := "p, someRole, applications, delete, */*, allow\ng, someUser, someRole"
	policyKey := "policy.csv"
	cm, _ := kubeclientset.CoreV1().ConfigMaps(fakeNamespace).Get(ctx, fakeConfigMapName, metav1.GetOptions{})

	t.Run("TestAddPolicySuccess", func(t *testing.T) {
		_, err := server.AddPolicy(ctx, &rbacpkg.RBACPolicyUpdateRequest{
			PolicyKey: policyKey,
			Policy: &application.RBACPolicyRule{
				Policy: policyRule,
			},
		})
		assert.Nil(t, err)
		cm, err := kubeclientset.CoreV1().ConfigMaps(fakeNamespace).Get(ctx, fakeConfigMapName, metav1.GetOptions{})
		assert.Nil(t, err)
		assert.Equal(t, policyRule, cm.Data[policyKey])
		assert.True(t, enf.Enforce("someUser", "applications", "delete", "*/*"))
	})

	delete(cm.Data, policyKey)
	_, _ = kubeclientset.CoreV1().ConfigMaps(fakeNamespace).Update(ctx, cm, metav1.UpdateOptions{})

	t.Run("TestAddPolicyFailure", func(t *testing.T) {
		_, err := server.AddPolicy(ctx, &rbacpkg.RBACPolicyUpdateRequest{
			PolicyKey: "invalid",
			Policy: &application.RBACPolicyRule{
				Policy: policyRule,
			},
		})
		assert.NotNil(t, err)
	})
	t.Run("TestAddDefaultPolicySuccess", func(t *testing.T) {
		_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
		_, err := server.AddPolicy(ctx, &rbacpkg.RBACPolicyUpdateRequest{
			PolicyKey: ConfigMapPolicyDefaultKey,
			Policy: &application.RBACPolicyRule{
				Policy: "role:readonly",
			},
		})
		assert.Nil(t, err)
		cm, err := kubeclientset.CoreV1().ConfigMaps(fakeNamespace).Get(ctx, fakeConfigMapName, metav1.GetOptions{})
		assert.Nil(t, err)
		assert.Equal(t, "role:readonly", cm.Data[ConfigMapPolicyDefaultKey])
		assert.True(t, enf.Enforce("someUser", "applications", "get", "*/*"))
	})

	delete(cm.Data, ConfigMapPolicyDefaultKey)
	_, _ = kubeclientset.CoreV1().ConfigMaps(fakeNamespace).Update(ctx, cm, metav1.UpdateOptions{})

	t.Run("TestAddPolicyWithOverlayKeySuccess", func(t *testing.T) {
		_, err := server.AddPolicy(ctx, &rbacpkg.RBACPolicyUpdateRequest{
			PolicyKey: "policy.someKey.csv",
			Policy: &application.RBACPolicyRule{
				Policy: policyRule,
			},
		})
		assert.Nil(t, err)
		cm, err := kubeclientset.CoreV1().ConfigMaps(fakeNamespace).Get(ctx, fakeConfigMapName, metav1.GetOptions{})
		assert.Nil(t, err)
		assert.Equal(t, policyRule, cm.Data["policy.someKey.csv"])
		assert.True(t, enf.Enforce("someUser", "applications", "delete", "*/*"))
	})

	delete(cm.Data, "policy.someKey.csv")
	_, _ = kubeclientset.CoreV1().ConfigMaps(fakeNamespace).Update(ctx, cm, metav1.UpdateOptions{})

	t.Run("TestRemovePolicySuccess", func(t *testing.T) {
		cm, err := kubeclientset.CoreV1().ConfigMaps(fakeNamespace).Get(ctx, fakeConfigMapName, metav1.GetOptions{})
		assert.Nil(t, err)
		cm.Data[policyKey] = policyRule
		_, err = kubeclientset.CoreV1().ConfigMaps(fakeNamespace).Update(ctx, cm, metav1.UpdateOptions{})
		assert.Nil(t, err)
		_, err = server.RemovePolicy(ctx, &rbacpkg.RBACPolicyQuery{
			PolicyKey: policyKey,
		})
		assert.Nil(t, err)
		cm, err = kubeclientset.CoreV1().ConfigMaps(fakeNamespace).Get(ctx, fakeConfigMapName, metav1.GetOptions{})
		assert.Nil(t, err)
		assert.Empty(t, cm.Data[policyKey])
		assert.False(t, enf.Enforce("someUser", "applications", "delete", "*/*"))
	})
	t.Run("RemovePolicyFailure", func(t *testing.T) {
		_, err := server.RemovePolicy(ctx, &rbacpkg.RBACPolicyQuery{
			PolicyKey: policyKey,
		})
		assert.NotNil(t, err)
	})
	t.Run("TestGetPolicy", func(t *testing.T) {
		cm, err := kubeclientset.CoreV1().ConfigMaps(fakeNamespace).Get(ctx, fakeConfigMapName, metav1.GetOptions{})
		assert.Nil(t, err)
		cm.Data[policyKey] = policyRule
		_, err = kubeclientset.CoreV1().ConfigMaps(fakeNamespace).Update(ctx, cm, metav1.UpdateOptions{})
		assert.Nil(t, err)
		policy, err := server.GetPolicy(ctx, &rbacpkg.RBACPolicyQuery{
			PolicyKey: policyKey,
		})
		assert.Nil(t, err)
		assert.Equal(t, policyRule, policy.Policy)
	})

	delete(cm.Data, policyKey)
	_, _ = kubeclientset.CoreV1().ConfigMaps(fakeNamespace).Update(ctx, cm, metav1.UpdateOptions{})

	t.Run("TestListPolicies", func(t *testing.T) {
		cm, err := kubeclientset.CoreV1().ConfigMaps(fakeNamespace).Get(ctx, fakeConfigMapName, metav1.GetOptions{})
		assert.Nil(t, err)
		cm.Data[policyKey] = policyRule
		_, err = kubeclientset.CoreV1().ConfigMaps(fakeNamespace).Update(ctx, cm, metav1.UpdateOptions{})
		assert.Nil(t, err)
		policies, err := server.ListPolicies(ctx, &rbacpkg.RBACPolicyListRequest{})
		assert.Nil(t, err)
		assert.Len(t, policies.Items, 1)
		assert.Equal(t, policyKey, policies.Items[0].PolicyKey)
		assert.Equal(t, policyRule, policies.Items[0].Policy)
		assert.NotEmpty(t, policies.Items[0])
	})
}
