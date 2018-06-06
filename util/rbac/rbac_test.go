package rbac

import (
	"context"
	"testing"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gobuffalo/packr"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	fakeConfgMapName  = "fake-cm"
	fakeNamespace     = "fake-ns"
	builtinPolicyFile = "builtin-policy.csv"
)

var box packr.Box

func init() {
	box = packr.NewBox(".")
}

func fakeConfigMap(policy ...string) *apiv1.ConfigMap {
	cm := apiv1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fakeConfgMapName,
			Namespace: fakeNamespace,
		},
		Data: make(map[string]string),
	}
	if len(policy) > 0 {
		cm.Data[ConfigMapPolicyCSVKey] = policy[0]
	}
	return &cm
}

// TestBuiltinPolicyEnforcer tests the builtin policy rules
func TestBuiltinPolicyEnforcer(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset()
	enf := NewEnforcer(kubeclientset, fakeNamespace, fakeConfgMapName, nil)
	err := enf.syncUpdate(fakeConfigMap())
	assert.Nil(t, err)

	// Without setting builtin policy, this should fail
	assert.False(t, enf.Enforce("admin", "applications", "get", "foo/bar"))

	// now set builtin policy
	enf.SetBuiltinPolicy(box.String(builtinPolicyFile))

	allowed := [][]interface{}{
		{"admin", "applications", "get", "foo/bar"},
		{"admin", "applications", "delete", "foo/bar"},
		{"role:readonly", "applications", "get", "foo/bar"},
		{"role:admin", "applications", "get", "foo/bar"},
		{"role:admin", "applications", "delete", "foo/bar"},
	}
	for _, a := range allowed {
		if !assert.True(t, enf.Enforce(a...)) {
			log.Errorf("%s: expected true, got false", a)
		}
	}

	disallowed := [][]interface{}{
		{"role:readonly", "applications", "create", "foo/bar"},
		{"role:readonly", "applications", "delete", "foo/bar"},
	}
	for _, a := range disallowed {
		if !assert.False(t, enf.Enforce(a...)) {
			log.Errorf("%s: expected false, got true", a)
		}
	}
}

// TestPolicyInformer verifies the informer will get updated with a new configmap
func TestPolicyInformer(t *testing.T) {
	cm := fakeConfigMap()
	kubeclientset := fake.NewSimpleClientset(cm)
	enf := NewEnforcer(kubeclientset, fakeNamespace, fakeConfgMapName, nil)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go enf.runInformer(ctx)

	// wait until the policy loads
	loaded := false
	for i := 1; i <= 20; i++ {
		if enf.Enforce("admin", "applications", "delete", "foo/bar") {
			loaded = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	assert.True(t, loaded)

	// update the configmap and update policy
	cm.Data[ConfigMapPolicyCSVKey] = "p, admin, applications, delete, */*"
	err := enf.syncUpdate(cm)
	assert.Nil(t, err)
	assert.True(t, enf.Enforce("admin", "applications", "delete", "foo/bar"))
}

// TestProjectIsolationEnforcement verifies the ability to create Project specific policies
func TestProjectIsolationEnforcement(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(fakeConfigMap())
	enf := NewEnforcer(kubeclientset, fakeNamespace, fakeConfgMapName, nil)
	policy := `
p, role:foo-admin, *, *, foo/*
p, role:bar-admin, *, *, bar/*
g, alice, role:foo-admin
g, bob, role:bar-admin
`
	enf.SetBuiltinPolicy(policy)

	// verify alice can only affect objects in foo and not bar,
	// and that bob can only affect objects in bar and not foo
	assert.True(t, enf.Enforce("bob", "applications", "delete", "bar/obj"))
	assert.False(t, enf.Enforce("bob", "applications", "delete", "foo/obj"))
	assert.True(t, enf.Enforce("alice", "applications", "delete", "foo/obj"))
	assert.False(t, enf.Enforce("alice", "applications", "delete", "bar/obj"))
}

// TestProjectReadOnly verifies the ability to have a read only role in a Project
func TestProjectReadOnly(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(fakeConfigMap())
	enf := NewEnforcer(kubeclientset, fakeNamespace, fakeConfgMapName, nil)
	policy := `
p, role:foo-readonly, *, get, foo/*
g, alice, role:foo-readonly
`
	enf.SetBuiltinPolicy(policy)

	assert.True(t, enf.Enforce("alice", "applications", "get", "foo/obj"))
	assert.False(t, enf.Enforce("alice", "applications", "delete", "bar/obj"))
	assert.False(t, enf.Enforce("alice", "applications", "get", "bar/obj"))
	assert.False(t, enf.Enforce("bob", "applications", "get", "foo/obj"))
}

func TestEnforceClaims(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(fakeConfigMap())
	enf := NewEnforcer(kubeclientset, fakeNamespace, fakeConfgMapName, nil)
	enf.SetBuiltinPolicy(box.String(builtinPolicyFile))
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

// TestDefaultRole tests the ability to set a default role
func TestDefaultRole(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset()
	enf := NewEnforcer(kubeclientset, fakeNamespace, fakeConfgMapName, nil)
	err := enf.syncUpdate(fakeConfigMap())
	assert.Nil(t, err)
	enf.SetBuiltinPolicy(box.String(builtinPolicyFile))
	claims := jwt.MapClaims{"groups": []string{"org1:team1", "org2:team2"}}

	assert.False(t, enf.Enforce("bob", "applications", "get", "foo/bar"))
	assert.False(t, enf.EnforceClaims(claims, "applications", "get", "foo/bar"))
	// after setting the default role to be the read-only role, this should now pass
	enf.SetDefaultRole("role:readonly")
	assert.True(t, enf.Enforce("bob", "applications", "get", "foo/bar"))
	assert.True(t, enf.EnforceClaims(claims, "applications", "get", "foo/bar"))
}

// TestURLAsObjectName tests the ability to have a URL as an object name
func TestURLAsObjectName(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset()
	enf := NewEnforcer(kubeclientset, fakeNamespace, fakeConfgMapName, nil)
	err := enf.syncUpdate(fakeConfigMap())
	assert.Nil(t, err)
	policy := `
p, alice, repositories, *, foo/*
p, bob, repositories, *, foo/https://github.com/argoproj/argo-cd.git
p, cathy, repositories, *, foo/*
`
	enf.SetUserPolicy(policy)

	assert.True(t, enf.Enforce("alice", "repositories", "delete", "foo/https://github.com/argoproj/argo-cd.git"))
	assert.True(t, enf.Enforce("alice", "repositories", "delete", "foo/https://github.com/golang/go.git"))

	assert.True(t, enf.Enforce("bob", "repositories", "delete", "foo/https://github.com/argoproj/argo-cd.git"))
	assert.False(t, enf.Enforce("bob", "repositories", "delete", "foo/https://github.com/golang/go.git"))

}

func TestEnforceNilClaims(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(fakeConfigMap())
	enf := NewEnforcer(kubeclientset, fakeNamespace, fakeConfgMapName, nil)
	enf.SetBuiltinPolicy(box.String(builtinPolicyFile))
	assert.False(t, enf.EnforceClaims(nil, "applications", "get", "foo/obj"))
	enf.SetDefaultRole("role:readonly")
	assert.True(t, enf.EnforceClaims(nil, "applications", "get", "foo/obj"))
}
