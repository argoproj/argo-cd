// +build !race

package rbac

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// TestPolicyInformer verifies the informer will get updated with a new configmap
func TestPolicyInformer(t *testing.T) {

	// !race:
	// A BUNCH of data race warnings thrown by running this test and the next... it's tough to guess to what degree this
	// is primarily a casbin issue or a Argo CD RBAC issue... A least one data race is an `rbac.go` with
	// itself, a bunch are in casbin. You can see the full list by doing a `go test -race github.com/argoproj/argo-cd/util/rbac`
	//
	// It couldn't hurt to take a look at this code to decide if Argo CD is properly handling concurrent data
	// access here, but in the mean time I have disabled data race testing of this test.

	cm := fakeConfigMap()
	cm.Data[ConfigMapPolicyCSVKey] = "p, admin, applications, delete, */*, allow"
	kubeclientset := fake.NewSimpleClientset(cm)
	enf := NewEnforcer(kubeclientset, fakeNamespace, fakeConfigMapName, nil)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go enf.runInformer(ctx, func(cm *apiv1.ConfigMap) error {
		return nil
	})

	loaded := false
	for i := 1; i <= 20; i++ {
		if enf.Enforce("admin", "applications", "delete", "foo/bar") {
			loaded = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	assert.True(t, loaded, "Policy update failed to load")

	// update the configmap and update policy
	delete(cm.Data, ConfigMapPolicyCSVKey)
	err := enf.syncUpdate(cm, noOpUpdate)
	assert.Nil(t, err)
	assert.False(t, enf.Enforce("admin", "applications", "delete", "foo/bar"))
}

// TestResourceActionWildcards verifies the ability to use wildcards in resources and actions
func TestResourceActionWildcards(t *testing.T) {

	// !race:
	// Same as TestPolicyInformer

	kubeclientset := fake.NewSimpleClientset(fakeConfigMap())
	enf := NewEnforcer(kubeclientset, fakeNamespace, fakeConfigMapName, nil)
	policy := `
p, alice, *, get, foo/obj, allow
p, bob, repositories, *, foo/obj, allow
p, cathy, *, *, foo/obj, allow
p, dave, applications, get, foo/obj, allow
p, dave, applications/*, get, foo/obj, allow
p, eve, *, get, foo/obj, deny
p, mallory, repositories, *, foo/obj, deny
p, mallory, repositories, *, foo/obj, allow
p, mike, *, *, foo/obj, allow
p, mike, *, *, foo/obj, deny
p, trudy, applications, get, foo/obj, allow
p, trudy, applications/*, get, foo/obj, allow
p, trudy, applications/secrets, get, foo/obj, deny
p, danny, applications, get, */obj, allow
p, danny, applications, get, proj1/a*p1, allow
`
	_ = enf.SetUserPolicy(policy)

	// Verify the resource wildcard
	assert.True(t, enf.Enforce("alice", "applications", "get", "foo/obj"))
	assert.True(t, enf.Enforce("alice", "applications/resources", "get", "foo/obj"))
	assert.False(t, enf.Enforce("alice", "applications/resources", "delete", "foo/obj"))

	// Verify action wildcards work
	assert.True(t, enf.Enforce("bob", "repositories", "get", "foo/obj"))
	assert.True(t, enf.Enforce("bob", "repositories", "delete", "foo/obj"))
	assert.False(t, enf.Enforce("bob", "applications", "get", "foo/obj"))

	// Verify resource and action wildcards work in conjunction
	assert.True(t, enf.Enforce("cathy", "repositories", "get", "foo/obj"))
	assert.True(t, enf.Enforce("cathy", "repositories", "delete", "foo/obj"))
	assert.True(t, enf.Enforce("cathy", "applications", "get", "foo/obj"))
	assert.True(t, enf.Enforce("cathy", "applications/resources", "delete", "foo/obj"))

	// Verify wildcards with sub-resources
	assert.True(t, enf.Enforce("dave", "applications", "get", "foo/obj"))
	assert.True(t, enf.Enforce("dave", "applications/logs", "get", "foo/obj"))

	// Verify the resource wildcard
	assert.False(t, enf.Enforce("eve", "applications", "get", "foo/obj"))
	assert.False(t, enf.Enforce("eve", "applications/resources", "get", "foo/obj"))
	assert.False(t, enf.Enforce("eve", "applications/resources", "delete", "foo/obj"))

	// Verify action wildcards work
	assert.False(t, enf.Enforce("mallory", "repositories", "get", "foo/obj"))
	assert.False(t, enf.Enforce("mallory", "repositories", "delete", "foo/obj"))
	assert.False(t, enf.Enforce("mallory", "applications", "get", "foo/obj"))

	// Verify resource and action wildcards work in conjunction
	assert.False(t, enf.Enforce("mike", "repositories", "get", "foo/obj"))
	assert.False(t, enf.Enforce("mike", "repositories", "delete", "foo/obj"))
	assert.False(t, enf.Enforce("mike", "applications", "get", "foo/obj"))
	assert.False(t, enf.Enforce("mike", "applications/resources", "delete", "foo/obj"))

	// Verify wildcards with sub-resources
	assert.True(t, enf.Enforce("trudy", "applications", "get", "foo/obj"))
	assert.True(t, enf.Enforce("trudy", "applications/logs", "get", "foo/obj"))
	assert.False(t, enf.Enforce("trudy", "applications/secrets", "get", "foo/obj"))

	// Verify trailing wildcards don't grant full access
	assert.True(t, enf.Enforce("danny", "applications", "get", "foo/obj"))
	assert.True(t, enf.Enforce("danny", "applications", "get", "bar/obj"))
	assert.False(t, enf.Enforce("danny", "applications", "get", "foo/bar"))
	assert.True(t, enf.Enforce("danny", "applications", "get", "proj1/app1"))
	assert.False(t, enf.Enforce("danny", "applications", "get", "proj1/app2"))
}
