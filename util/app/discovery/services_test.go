package discovery

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"
)

func getNameMock(p *plugin) string {
	return p.owner.portName
}

var mockPlugins = []*plugin {
	{
		name: `foo-1.0`,
			pluginType: service,
			address: `bar.ns.svc.cluster.local:9000`,
			owner: pluginOwner{ namespace:`ns`, serviceName: `bar` },
		},
		{
		name: `foo-1.1`,
			pluginType: service,
			address: `bar.ns.svc.cluster.local:9001`,
			owner: pluginOwner{ namespace:`ns`, serviceName: `bar` },
		},
		{
		name: `banana`,
			pluginType: service,
			address: `fruit.ns.svc.cluster.local:9000`,
			owner: pluginOwner{ namespace:`ns`, serviceName: `fruit` },
		},
		{
		name: `banana`,
			pluginType: service,
			address: `fruit.otherns.svc.cluster.local:9000`,
			owner: pluginOwner{ namespace:`otherns`, serviceName: `fruit` },
		},
	}

func verifySvcExists(t *testing.T, plugins *plugins, name, address string) {
	t.Helper()
	found := false
	for _, p := range plugins.servicePlugins {
		if p.name == name {
			found = true
			assert.Equal(t, service, p.pluginType)
			assert.Equal(t, address, p.address)
		}
	}
	assert.True(t, found, fmt.Sprintf("Service %s not found when expected", name))
}

func verifySvcMissing(t *testing.T, plugins *plugins, name string) {
	t.Helper()
	found := false
	for _, p := range plugins.servicePlugins {
		if p.name == name {
			found = true
		}
	}
	assert.False(t, found, fmt.Sprintf("Service %s found when not expected", name))
}

func TestServiceChanges(t *testing.T) {
	testPlugins := plugins{
		servicePlugins: mockPlugins,
		getName: getNameMock,
	}
	assert.Len(t, testPlugins.servicePlugins, 4)
	svc1 := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: `testsvc`,
			Namespace: `testns`,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Name: `testport1`,
				Port: 123,
			},{
				Name: `testport2`,
				Port: 234,
			}},
		},
	}
	testPlugins.svcAdd(svc1)
	assert.Len(t, testPlugins.servicePlugins, 6)
	verifySvcExists(t, &testPlugins, `testport1`, `testsvc.testns.svc.cluster.local:123`)
	verifySvcExists(t, &testPlugins, `testport2`, `testsvc.testns.svc.cluster.local:234`)

	// Delete two of the original plugins
	testPlugins.svcDelete(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: `bar`,
			Namespace: `ns`,
		},
	})
	assert.Len(t, testPlugins.servicePlugins, 4)
	verifySvcMissing(t, &testPlugins, `foo-1.0`)
	verifySvcMissing(t, &testPlugins, `foo-1.1`)

	// Replace with identical
	testPlugins.svcUpdate(svc1, svc1)
	assert.Len(t, testPlugins.servicePlugins, 4)
	verifySvcExists(t, &testPlugins, `testport1`, `testsvc.testns.svc.cluster.local:123`)
	verifySvcExists(t, &testPlugins, `testport2`, `testsvc.testns.svc.cluster.local:234`)

	// Modify the service
	svc2 := svc1.DeepCopy()
	svc2.Spec.Ports[1].Name = `testport3`
	svc2.Spec.Ports[1].Port = 456
	testPlugins.svcUpdate(svc1, svc2)
	assert.Len(t, testPlugins.servicePlugins, 4)
	verifySvcExists(t, &testPlugins, `testport1`, `testsvc.testns.svc.cluster.local:123`)
	verifySvcMissing(t, &testPlugins, `testport2`)
	verifySvcExists(t, &testPlugins, `testport3`, `testsvc.testns.svc.cluster.local:456`)

	// Modify it back and append a new port
	svc3 := svc1.DeepCopy()
	svc3.Spec.Ports = append(svc3.Spec.Ports,
		corev1.ServicePort{
			Name: `testport4`,
			Port: 999,
		})
	testPlugins.svcUpdate(svc2, svc3)
	assert.Len(t, testPlugins.servicePlugins, 5)
	verifySvcExists(t, &testPlugins, `testport1`, `testsvc.testns.svc.cluster.local:123`)
	verifySvcExists(t, &testPlugins, `testport2`, `testsvc.testns.svc.cluster.local:234`)
	verifySvcMissing(t, &testPlugins, `testport3`)
	verifySvcExists(t, &testPlugins, `testport4`, `testsvc.testns.svc.cluster.local:999`)

	// Delete new service
	testPlugins.svcDelete(svc3)
	assert.Len(t, testPlugins.servicePlugins, 2)
	verifySvcMissing(t, &testPlugins, `testport1`)
	verifySvcMissing(t, &testPlugins, `testport2`)
	verifySvcMissing(t, &testPlugins, `testport4`)
}
