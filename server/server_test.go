package server

import (
	"fmt"
	"testing"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	apps "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/util/rbac"
	jwt "github.com/dgrijalva/jwt-go"
	log "github.com/sirupsen/logrus"
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
	roleName := "testRole"
	subFormat := "proj:%s:%s"
	policyTemplate := "p, %s, applications, get, %s/%s, %s"

	defaultObject := "*"
	defaultEffect := "allow"
	defaultTestObject := fmt.Sprintf("%s/%s", projectName, "test")
	defaultCreatedAt := int64(1)
	defaultSub := fmt.Sprintf(subFormat, projectName, roleName)
	defaultPolicy := fmt.Sprintf(policyTemplate, defaultSub, projectName, defaultObject, defaultEffect)

	role := v1alpha1.ProjectRole{Name: roleName, Policies: []string{defaultPolicy}, JwtTokens: []v1alpha1.JwtToken{{CreatedAt: defaultCreatedAt}}}
	existingProj := v1alpha1.AppProject{
		ObjectMeta: v1.ObjectMeta{Name: projectName, Namespace: fakeNamespace},
		Spec: v1alpha1.AppProjectSpec{
			Roles: []v1alpha1.ProjectRole{role},
		},
	}
	cm := fakeConfigMap()
	secret := fakeSecret()
	kubeclientset := fake.NewSimpleClientset(cm, secret)

	t.Run("TestEnforceJwtTokenSuccessful", func(t *testing.T) {
		s := NewServer(ArgoCDServerOpts{Namespace: fakeNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(&existingProj)})
		s.newGRPCServer()
		claims := jwt.MapClaims{"sub": defaultSub, "iat": defaultCreatedAt}
		assert.True(t, s.enf.EnforceClaims(claims, "applications", "get", defaultTestObject))
	})

	t.Run("TestEnforceJwtTokenWithDiffCreateAtFailure", func(t *testing.T) {
		s := NewServer(ArgoCDServerOpts{Namespace: fakeNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(&existingProj)})
		s.newGRPCServer()
		diffCreateAt := defaultCreatedAt + 1
		claims := jwt.MapClaims{"sub": defaultSub, "iat": diffCreateAt}
		assert.False(t, s.enf.EnforceClaims(claims, "applications", "get", defaultTestObject))
	})

	t.Run("TestEnforceJwtTokenIncorrectSubFormatFailure", func(t *testing.T) {
		s := NewServer(ArgoCDServerOpts{Namespace: fakeNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(&existingProj)})
		s.newGRPCServer()
		invalidSub := "proj:test"
		claims := jwt.MapClaims{"sub": invalidSub, "iat": defaultCreatedAt}
		assert.False(t, s.enf.EnforceClaims(claims, "applications", "get", defaultTestObject))
	})

	t.Run("TestEnforceJwtTokenNoTokenFailure", func(t *testing.T) {
		s := NewServer(ArgoCDServerOpts{Namespace: fakeNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(&existingProj)})
		s.newGRPCServer()
		nonExistentToken := "fake-token"
		invalidSub := fmt.Sprintf(subFormat, projectName, nonExistentToken)
		claims := jwt.MapClaims{"sub": invalidSub, "iat": defaultCreatedAt}

		assert.False(t, s.enf.EnforceClaims(claims, "applications", "get", defaultTestObject))
	})

	t.Run("TestEnforceJwtTokenNotJwtTokenFailure", func(t *testing.T) {
		proj := existingProj.DeepCopy()
		proj.Spec.Roles[0].JwtTokens = nil
		s := NewServer(ArgoCDServerOpts{Namespace: fakeNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(proj)})
		s.newGRPCServer()
		claims := jwt.MapClaims{"sub": defaultSub, "iat": defaultCreatedAt}
		assert.False(t, s.enf.EnforceClaims(claims, "applications", "get", defaultTestObject))
	})

	t.Run("TestEnforceJwtTokenExplicitDeny", func(t *testing.T) {
		denyApp := "testDenyApp"
		allowPolicy := fmt.Sprintf(policyTemplate, defaultSub, projectName, defaultObject, defaultEffect)
		denyPolicy := fmt.Sprintf(policyTemplate, defaultSub, projectName, denyApp, "deny")
		role := v1alpha1.ProjectRole{Name: roleName, Policies: []string{allowPolicy, denyPolicy}, JwtTokens: []v1alpha1.JwtToken{{CreatedAt: defaultCreatedAt}}}
		proj := existingProj.DeepCopy()
		proj.Spec.Roles[0] = role

		s := NewServer(ArgoCDServerOpts{Namespace: fakeNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(proj)})
		s.newGRPCServer()

		claims := jwt.MapClaims{"sub": defaultSub, "iat": defaultCreatedAt}
		allowedObject := fmt.Sprintf("%s/%s", projectName, "test")
		denyObject := fmt.Sprintf("%s/%s", projectName, denyApp)
		assert.True(t, s.enf.EnforceClaims(claims, "applications", "get", allowedObject))
		assert.False(t, s.enf.EnforceClaims(claims, "applications", "get", denyObject))
	})
}

func TestEnforceClaims(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(fakeConfigMap())

	enf := rbac.NewEnforcer(kubeclientset, fakeNamespace, common.ArgoCDConfigMapName, nil)
	enf.SetBuiltinPolicy(box.String(builtinPolicyFile))
	enf.SetClaimsEnforcerFunc(DefaultEnforceClaims(enf, nil, fakeNamespace))
	policy := `
g, org2:team2, role:admin
g, bob, role:admin
`
	enf.SetUserPolicy(policy)
	allowed := []jwt.Claims{
		jwt.MapClaims{"groups": []string{"org1:team1", "org2:team2"}},
		jwt.StandardClaims{Subject: "admin"},
	}
	for _, c := range allowed {
		if !assert.True(t, enf.EnforceClaims(c, "applications", "delete", "foo/obj")) {
			log.Errorf("%v: expected true, got false", c)
		}
	}

	disallowed := []jwt.Claims{
		jwt.MapClaims{"groups": []string{"org3:team3"}},
		jwt.StandardClaims{Subject: "nobody"},
	}
	for _, c := range disallowed {
		if !assert.False(t, enf.EnforceClaims(c, "applications", "delete", "foo/obj")) {
			log.Errorf("%v: expected true, got false", c)
		}
	}
}

func TestDefaultRoleWithClaims(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset()
	enf := rbac.NewEnforcer(kubeclientset, fakeNamespace, common.ArgoCDConfigMapName, nil)
	enf.SetBuiltinPolicy(box.String(builtinPolicyFile))
	enf.SetClaimsEnforcerFunc(DefaultEnforceClaims(enf, nil, fakeNamespace))
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
	enf.SetClaimsEnforcerFunc(DefaultEnforceClaims(enf, nil, fakeNamespace))
	assert.False(t, enf.EnforceClaims(nil, "applications", "get", "foo/obj"))
	enf.SetDefaultRole("role:readonly")
	assert.True(t, enf.EnforceClaims(nil, "applications", "get", "foo/obj"))
}
