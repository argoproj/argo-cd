package rbac

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dgrijalva/jwt-go"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/util/assets"
)

const (
	fakeConfigMapName = "fake-cm"
	fakeNamespace     = "fake-ns"
)

var (
	noOpUpdate = func(cm *apiv1.ConfigMap) error {
		return nil
	}
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

// TestBuiltinPolicyEnforcer tests the builtin policy rules
func TestBuiltinPolicyEnforcer(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset()
	enf := NewEnforcer(kubeclientset, fakeNamespace, fakeConfigMapName, nil)
	err := enf.syncUpdate(fakeConfigMap(), noOpUpdate)
	assert.Nil(t, err)

	// Without setting builtin policy, this should fail
	assert.False(t, enf.Enforce("admin", "applications", "get", "foo/bar"))

	// now set builtin policy
	_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)

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

// TestProjectIsolationEnforcement verifies the ability to create Project specific policies
func TestProjectIsolationEnforcement(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(fakeConfigMap())
	enf := NewEnforcer(kubeclientset, fakeNamespace, fakeConfigMapName, nil)
	policy := `
p, role:foo-admin, *, *, foo/*, allow
p, role:bar-admin, *, *, bar/*, allow
g, alice, role:foo-admin
g, bob, role:bar-admin
`
	_ = enf.SetBuiltinPolicy(policy)

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
	enf := NewEnforcer(kubeclientset, fakeNamespace, fakeConfigMapName, nil)
	policy := `
p, role:foo-readonly, *, get, foo/*, allow
g, alice, role:foo-readonly
`
	_ = enf.SetBuiltinPolicy(policy)

	assert.True(t, enf.Enforce("alice", "applications", "get", "foo/obj"))
	assert.False(t, enf.Enforce("alice", "applications", "delete", "bar/obj"))
	assert.False(t, enf.Enforce("alice", "applications", "get", "bar/obj"))
	assert.False(t, enf.Enforce("bob", "applications", "get", "foo/obj"))
}

// TestDefaultRole tests the ability to set a default role
func TestDefaultRole(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset()
	enf := NewEnforcer(kubeclientset, fakeNamespace, fakeConfigMapName, nil)
	err := enf.syncUpdate(fakeConfigMap(), noOpUpdate)
	assert.Nil(t, err)
	_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)

	assert.False(t, enf.Enforce("bob", "applications", "get", "foo/bar"))
	// after setting the default role to be the read-only role, this should now pass
	enf.SetDefaultRole("role:readonly")
	assert.True(t, enf.Enforce("bob", "applications", "get", "foo/bar"))
}

// TestURLAsObjectName tests the ability to have a URL as an object name
func TestURLAsObjectName(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset()
	enf := NewEnforcer(kubeclientset, fakeNamespace, fakeConfigMapName, nil)
	err := enf.syncUpdate(fakeConfigMap(), noOpUpdate)
	assert.Nil(t, err)
	policy := `
p, alice, repositories, *, foo/*, allow
p, bob, repositories, *, foo/https://github.com/argoproj/argo-cd.git, allow
p, cathy, repositories, *, foo/*, allow
`
	_ = enf.SetUserPolicy(policy)

	assert.True(t, enf.Enforce("alice", "repositories", "delete", "foo/https://github.com/argoproj/argo-cd.git"))
	assert.True(t, enf.Enforce("alice", "repositories", "delete", "foo/https://github.com/golang/go.git"))

	assert.True(t, enf.Enforce("bob", "repositories", "delete", "foo/https://github.com/argoproj/argo-cd.git"))
	assert.False(t, enf.Enforce("bob", "repositories", "delete", "foo/https://github.com/golang/go.git"))

}

func TestEnableDisableEnforce(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(fakeConfigMap())
	enf := NewEnforcer(kubeclientset, fakeNamespace, fakeConfigMapName, nil)
	policy := `
p, alice, *, get, foo/obj, allow
p, mike, *, get, foo/obj, deny
`
	_ = enf.SetUserPolicy(policy)
	enf.SetClaimsEnforcerFunc(func(claims jwt.Claims, rvals ...interface{}) bool {
		return false
	})

	assert.True(t, enf.Enforce("alice", "applications", "get", "foo/obj"))
	assert.False(t, enf.Enforce("alice", "applications/resources", "delete", "foo/obj"))
	assert.False(t, enf.Enforce("mike", "applications", "get", "foo/obj"))
	assert.False(t, enf.Enforce("mike", "applications/resources", "delete", "foo/obj"))
	assert.False(t, enf.Enforce(nil, "applications/resources", "delete", "foo/obj"))
	assert.False(t, enf.Enforce(&jwt.StandardClaims{}, "applications/resources", "delete", "foo/obj"))

	enf.EnableEnforce(false)
	assert.True(t, enf.Enforce("alice", "applications", "get", "foo/obj"))
	assert.True(t, enf.Enforce("alice", "applications/resources", "delete", "foo/obj"))
	assert.True(t, enf.Enforce("mike", "applications", "get", "foo/obj"))
	assert.True(t, enf.Enforce("mike", "applications/resources", "delete", "foo/obj"))
	assert.True(t, enf.Enforce(nil, "applications/resources", "delete", "foo/obj"))
	assert.True(t, enf.Enforce(&jwt.StandardClaims{}, "applications/resources", "delete", "foo/obj"))
}

func TestUpdatePolicy(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(fakeConfigMap())
	enf := NewEnforcer(kubeclientset, fakeNamespace, fakeConfigMapName, nil)

	_ = enf.SetUserPolicy("p, alice, *, get, foo/obj, allow")
	assert.True(t, enf.Enforce("alice", "applications", "get", "foo/obj"))
	assert.False(t, enf.Enforce("bob", "applications", "get", "foo/obj"))

	_ = enf.SetUserPolicy("p, bob, *, get, foo/obj, allow")
	assert.False(t, enf.Enforce("alice", "applications", "get", "foo/obj"))
	assert.True(t, enf.Enforce("bob", "applications", "get", "foo/obj"))

	_ = enf.SetUserPolicy("")
	assert.False(t, enf.Enforce("alice", "applications", "get", "foo/obj"))
	assert.False(t, enf.Enforce("bob", "applications", "get", "foo/obj"))

	_ = enf.SetBuiltinPolicy("p, alice, *, get, foo/obj, allow")
	assert.True(t, enf.Enforce("alice", "applications", "get", "foo/obj"))
	assert.False(t, enf.Enforce("bob", "applications", "get", "foo/obj"))

	_ = enf.SetBuiltinPolicy("p, bob, *, get, foo/obj, allow")
	assert.False(t, enf.Enforce("alice", "applications", "get", "foo/obj"))
	assert.True(t, enf.Enforce("bob", "applications", "get", "foo/obj"))

	_ = enf.SetBuiltinPolicy("")
	assert.False(t, enf.Enforce("alice", "applications", "get", "foo/obj"))
	assert.False(t, enf.Enforce("bob", "applications", "get", "foo/obj"))
}

func TestNoPolicy(t *testing.T) {
	cm := fakeConfigMap()
	kubeclientset := fake.NewSimpleClientset(cm)
	enf := NewEnforcer(kubeclientset, fakeNamespace, fakeConfigMapName, nil)
	assert.False(t, enf.Enforce("admin", "applications", "delete", "foo/bar"))
}

// TestClaimsEnforcerFunc tests
func TestClaimsEnforcerFunc(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset()
	enf := NewEnforcer(kubeclientset, fakeNamespace, fakeConfigMapName, nil)
	claims := jwt.StandardClaims{
		Subject: "foo",
	}
	assert.False(t, enf.Enforce(&claims, "applications", "get", "foo/bar"))
	enf.SetClaimsEnforcerFunc(func(claims jwt.Claims, rvals ...interface{}) bool {
		return true
	})
	assert.True(t, enf.Enforce(&claims, "applications", "get", "foo/bar"))
}

// TestDefaultRoleWithRuntimePolicy tests the ability for a default role to still take affect when
// enforcing a runtime policy
func TestDefaultRoleWithRuntimePolicy(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset()
	enf := NewEnforcer(kubeclientset, fakeNamespace, fakeConfigMapName, nil)
	err := enf.syncUpdate(fakeConfigMap(), noOpUpdate)
	assert.Nil(t, err)
	runtimePolicy := assets.BuiltinPolicyCSV
	assert.False(t, enf.EnforceRuntimePolicy(runtimePolicy, "bob", "applications", "get", "foo/bar"))
	enf.SetDefaultRole("role:readonly")
	assert.True(t, enf.EnforceRuntimePolicy(runtimePolicy, "bob", "applications", "get", "foo/bar"))
}

// TestClaimsEnforcerFuncWithRuntimePolicy tests the ability for claims enforcer function to still
// take effect when enforcing a runtime policy
func TestClaimsEnforcerFuncWithRuntimePolicy(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset()
	enf := NewEnforcer(kubeclientset, fakeNamespace, fakeConfigMapName, nil)
	err := enf.syncUpdate(fakeConfigMap(), noOpUpdate)
	assert.Nil(t, err)
	runtimePolicy := assets.BuiltinPolicyCSV
	claims := jwt.StandardClaims{
		Subject: "foo",
	}
	assert.False(t, enf.EnforceRuntimePolicy(runtimePolicy, claims, "applications", "get", "foo/bar"))
	enf.SetClaimsEnforcerFunc(func(claims jwt.Claims, rvals ...interface{}) bool {
		return true
	})
	assert.True(t, enf.EnforceRuntimePolicy(runtimePolicy, claims, "applications", "get", "foo/bar"))
}

// TestInvalidRuntimePolicy tests when an invalid policy is supplied, it falls back to normal enforcement
func TestInvalidRuntimePolicy(t *testing.T) {
	cm := fakeConfigMap()
	kubeclientset := fake.NewSimpleClientset(cm)
	enf := NewEnforcer(kubeclientset, fakeNamespace, fakeConfigMapName, nil)
	err := enf.syncUpdate(fakeConfigMap(), noOpUpdate)
	assert.Nil(t, err)
	_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
	assert.True(t, enf.EnforceRuntimePolicy("", "admin", "applications", "update", "foo/bar"))
	assert.False(t, enf.EnforceRuntimePolicy("", "role:readonly", "applications", "update", "foo/bar"))
	badPolicy := "this, is, not, a, good, policy"
	assert.True(t, enf.EnforceRuntimePolicy(badPolicy, "admin", "applications", "update", "foo/bar"))
	assert.False(t, enf.EnforceRuntimePolicy(badPolicy, "role:readonly", "applications", "update", "foo/bar"))
}

func TestValidatePolicy(t *testing.T) {
	goodPolicies := []string{
		"p, role:admin, projects, delete, *, allow",
		"",
		"#",
		`p, "role,admin", projects, delete, *, allow`,
		` p, role:admin, projects, delete, *, allow `,
	}
	for _, good := range goodPolicies {
		assert.Nil(t, ValidatePolicy(good))
	}
	badPolicies := []string{
		"this, is, not, a, good, policy",
		"this\ttoo",
	}
	for _, bad := range badPolicies {
		assert.Error(t, ValidatePolicy(bad))
	}
}

// TestEnforceErrorMessage ensures we give descriptive error message
func TestEnforceErrorMessage(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset()
	enf := NewEnforcer(kubeclientset, fakeNamespace, fakeConfigMapName, nil)
	err := enf.syncUpdate(fakeConfigMap(), noOpUpdate)
	assert.Nil(t, err)

	err = enf.EnforceErr("admin", "applications", "get", "foo/bar")
	assert.Error(t, err)
	assert.Equal(t, "rpc error: code = PermissionDenied desc = permission denied: applications, get, foo/bar", err.Error())

	err = enf.EnforceErr()
	assert.Error(t, err)
	assert.Equal(t, "rpc error: code = PermissionDenied desc = permission denied", err.Error())

	// nolint:staticcheck
	ctx := context.WithValue(context.Background(), "claims", &jwt.StandardClaims{Subject: "proj:default:admin"})
	err = enf.EnforceErr(ctx.Value("claims"), "project")
	assert.Error(t, err)
	assert.Equal(t, "rpc error: code = PermissionDenied desc = permission denied: project, sub: proj:default:admin", err.Error())

	iat := time.Unix(int64(1593035962), 0).Format(time.RFC3339)
	exp := fmt.Sprintf("rpc error: code = PermissionDenied desc = permission denied: project, sub: proj:default:admin, iat: %s", iat)
	// nolint:staticcheck
	ctx = context.WithValue(context.Background(), "claims", &jwt.StandardClaims{Subject: "proj:default:admin", IssuedAt: 1593035962})
	err = enf.EnforceErr(ctx.Value("claims"), "project")
	assert.Error(t, err)
	assert.Equal(t, exp, err.Error())

	// nolint:staticcheck
	ctx = context.WithValue(context.Background(), "claims", &jwt.StandardClaims{ExpiresAt: 1})
	err = enf.EnforceErr(ctx.Value("claims"), "project")
	assert.Error(t, err)
	assert.Equal(t, "rpc error: code = PermissionDenied desc = permission denied: project", err.Error())

	// nolint:staticcheck
	ctx = context.WithValue(context.Background(), "claims", &jwt.StandardClaims{Subject: "proj:default:admin", IssuedAt: 0})
	err = enf.EnforceErr(ctx.Value("claims"), "project")
	assert.Error(t, err)
	assert.Equal(t, "rpc error: code = PermissionDenied desc = permission denied: project, sub: proj:default:admin", err.Error())

}
