package server

import (
	"fmt"
	"testing"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	apps "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/util/rbac"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes/fake"
)

const (
	fakeNamespace     = "fake-ns"
	builtinPolicyFile = "builtin-policy.csv"
)

func fakeConfigMap() *apiv1.ConfigMap {
	cm := apiv1.ConfigMap{
		TypeMeta: v1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      common.ArgoCDConfigMapName,
			Namespace: fakeNamespace,
		},
		Data: make(map[string]string),
	}
	return &cm
}

func fakeSecret(policy ...string) *apiv1.Secret {
	secret := apiv1.Secret{
		TypeMeta: v1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      common.ArgoCDSecretName,
			Namespace: fakeNamespace,
		},
		Data: make(map[string][]byte),
	}
	return &secret
}

func TestEnforceJwtToken(t *testing.T) {
	projectName := "testProj"
	tokenName := "testToken"
	subFormat := "proj:%s:%s"
	sub := fmt.Sprintf(subFormat, projectName, tokenName)
	policy := fmt.Sprintf("p, %s, projects, get, %s", sub, projectName)
	createdAt := int64(1)

	token := v1alpha1.ProjectRole{Name: tokenName, Policies: []string{policy}, JwtToken: &v1alpha1.JwtToken{CreatedAt: createdAt}}
	existingProj := v1alpha1.AppProject{
		ObjectMeta: v1.ObjectMeta{Name: projectName, Namespace: fakeNamespace},
		Spec: v1alpha1.AppProjectSpec{
			Roles: []v1alpha1.ProjectRole{token},
		},
	}
	cm := fakeConfigMap()
	secret := fakeSecret()
	kubeclientset := fake.NewSimpleClientset(cm, secret)

	t.Run("TestEnforceJwtTokenSuccessful", func(t *testing.T) {
		s := NewServer(ArgoCDServerOpts{Namespace: fakeNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(&existingProj)})
		s.newGRPCServer()
		claims := jwt.MapClaims{"sub": sub, "iat": createdAt}
		assert.True(t, s.enf.EnforceClaims(claims, "projects", "get", projectName))
	})

	t.Run("TestEnforceJwtTokenWithDiffCreateAtFailure", func(t *testing.T) {
		s := NewServer(ArgoCDServerOpts{Namespace: fakeNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(&existingProj)})
		s.newGRPCServer()
		diffCreateAt := createdAt + 1
		claims := jwt.MapClaims{"sub": sub, "iat": diffCreateAt}
		assert.False(t, s.enf.EnforceClaims(claims, "projects", "get", projectName))
	})

	t.Run("TestEnforceJwtTokenIncorrectSubFormatFailure", func(t *testing.T) {
		s := NewServer(ArgoCDServerOpts{Namespace: fakeNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(&existingProj)})
		s.newGRPCServer()
		invalidSub := "proj:test"
		claims := jwt.MapClaims{"sub": invalidSub, "iat": createdAt}
		assert.False(t, s.enf.EnforceClaims(claims, "projects", "get", projectName))
	})

	t.Run("TestEnforceJwtTokenNoTokenFailure", func(t *testing.T) {
		s := NewServer(ArgoCDServerOpts{Namespace: fakeNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(&existingProj)})
		s.newGRPCServer()
		nonExistentToken := "fake-token"
		invalidSub := fmt.Sprintf(subFormat, projectName, nonExistentToken)
		claims := jwt.MapClaims{"sub": invalidSub, "iat": createdAt}
		assert.False(t, s.enf.EnforceClaims(claims, "projects", "get", projectName))
	})

	t.Run("TestEnforceJwtTokenNotJwtTokenFailure", func(t *testing.T) {
		proj := existingProj.DeepCopy()
		proj.Spec.Roles[0].JwtToken = nil
		s := NewServer(ArgoCDServerOpts{Namespace: fakeNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(proj)})
		s.newGRPCServer()
		claims := jwt.MapClaims{"sub": sub, "iat": createdAt}
		assert.False(t, s.enf.EnforceClaims(claims, "projects", "get", projectName))
	})
}
func TestDefaultRoleWithClaims(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset()
	enf := rbac.NewEnforcer(kubeclientset, fakeNamespace, common.ArgoCDConfigMapName, nil)
	enf.SetBuiltinPolicy(box.String(builtinPolicyFile))
	enf.SetClaimsEnforcerFunc(defaultEnforceClaims(enf, nil, fakeNamespace))
	claims := jwt.MapClaims{"groups": []string{"org1:team1", "org2:team2"}}

	assert.False(t, enf.EnforceClaims(claims, "applications", "get", "foo/bar"))
	// after setting the default role to be the read-only role, this should now pass
	enf.SetDefaultRole("role:readonly")
	assert.True(t, enf.EnforceClaims(claims, "applications", "get", "foo/bar"))
}

func TestEnforceNilClaims(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(fakeConfigMap())
	enf := rbac.NewEnforcer(kubeclientset, fakeNamespace, common.ArgoCDConfigMapName, nil)
	enf.SetBuiltinPolicy(box.String(builtinPolicyFile))
	enf.SetClaimsEnforcerFunc(defaultEnforceClaims(enf, nil, fakeNamespace))
	assert.False(t, enf.EnforceClaims(nil, "applications", "get", "foo/obj"))
	enf.SetDefaultRole("role:readonly")
	assert.True(t, enf.EnforceClaims(nil, "applications", "get", "foo/obj"))
}
