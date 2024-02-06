package kube

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakedisco "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/rest"
	testcore "k8s.io/client-go/testing"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/yaml"
)

const depWithLabel = `
apiVersion: extensions/v1beta2
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    foo: bar
spec:
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - image: nginx:1.7.9
        name: nginx
        ports:
        - containerPort: 80
`

var standardVerbs = v1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"}

func TestUnsetLabels(t *testing.T) {
	for _, yamlStr := range [][]byte{[]byte(depWithLabel)} {
		var obj unstructured.Unstructured
		err := yaml.Unmarshal(yamlStr, &obj)
		require.NoError(t, err)

		UnsetLabel(&obj, "foo")

		manifestBytes, err := json.MarshalIndent(obj.Object, "", "  ")
		require.NoError(t, err)

		var dep extv1beta1.Deployment
		err = json.Unmarshal(manifestBytes, &dep)
		require.NoError(t, err)
		assert.Empty(t, dep.ObjectMeta.Labels)
	}

}

func TestCleanKubectlOutput(t *testing.T) {
	{
		s := `error: error validating "STDIN": error validating data: ValidationError(Deployment.spec): missing required field "selector" in io.k8s.api.apps.v1beta2.DeploymentSpec; if you choose to ignore these errors, turn validation off with --validate=false`
		assert.Equal(t, cleanKubectlOutput(s), `error validating data: ValidationError(Deployment.spec): missing required field "selector" in io.k8s.api.apps.v1beta2.DeploymentSpec`)
	}
	{
		s := `error when applying patch:
{"metadata":{"annotations":{"kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"v1\",\"kind\":\"Service\",\"metadata\":{\"annotations\":{},\"labels\":{\"app.kubernetes.io/instance\":\"test-immutable-change\"},\"name\":\"my-service\",\"namespace\":\"argocd-e2e--test-immutable-change-ysfud\"},\"spec\":{\"clusterIP\":\"10.96.0.44\",\"ports\":[{\"port\":80,\"protocol\":\"TCP\",\"targetPort\":9376}],\"selector\":{\"app\":\"MyApp\"}}}\n"}},"spec":{"clusterIP":"10.96.0.44"}}
to:
Resource: "/v1, Resource=services", GroupVersionKind: "/v1, Kind=Service"
Name: "my-service", Namespace: "argocd-e2e--test-immutable-change-ysfud"
Object: &{map["apiVersion":"v1" "kind":"Service" "metadata":map["annotations":map["kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"v1\",\"kind\":\"Service\",\"metadata\":{\"annotations\":{},\"labels\":{\"app.kubernetes.io/instance\":\"test-immutable-change\"},\"name\":\"my-service\",\"namespace\":\"argocd-e2e--test-immutable-change-ysfud\"},\"spec\":{\"clusterIP\":\"10.96.0.43\",\"ports\":[{\"port\":80,\"protocol\":\"TCP\",\"targetPort\":9376}],\"selector\":{\"app\":\"MyApp\"}}}\n"] "creationTimestamp":"2019-12-11T15:29:56Z" "labels":map["app.kubernetes.io/instance":"test-immutable-change"] "name":"my-service" "namespace":"argocd-e2e--test-immutable-change-ysfud" "resourceVersion":"157426" "selfLink":"/api/v1/namespaces/argocd-e2e--test-immutable-change-ysfud/services/my-service" "uid":"339cf96f-47eb-4759-ac95-30a169dce004"] "spec":map["clusterIP":"10.96.0.43" "ports":[map["port":'P' "protocol":"TCP" "targetPort":'\u24a0']] "selector":map["app":"MyApp"] "sessionAffinity":"None" "type":"ClusterIP"] "status":map["loadBalancer":map[]]]}
for: "/var/folders/_m/991sn1ds7g39lnbhp6wvqp9d_j5476/T/224503547": Service "my-service" is invalid: spec.clusterIP: Invalid value: "10.96.0.44": field is immutable`
		assert.Equal(t, cleanKubectlOutput(s), `Service "my-service" is invalid: spec.clusterIP: Invalid value: "10.96.0.44": field is immutable`)
	}
}

func TestInClusterKubeConfig(t *testing.T) {
	restConfig := &rest.Config{}
	kubeConfig := NewKubeConfig(restConfig, "")
	assert.NotEmpty(t, kubeConfig.AuthInfos[kubeConfig.CurrentContext].TokenFile)

	restConfig = &rest.Config{
		Password: "foo",
	}
	kubeConfig = NewKubeConfig(restConfig, "")
	assert.Empty(t, kubeConfig.AuthInfos[kubeConfig.CurrentContext].TokenFile)

	restConfig = &rest.Config{
		ExecProvider: &clientcmdapi.ExecConfig{
			APIVersion: "client.authentication.k8s.io/v1alpha1",
			Command:    "aws",
		},
	}
	kubeConfig = NewKubeConfig(restConfig, "")
	assert.Empty(t, kubeConfig.AuthInfos[kubeConfig.CurrentContext].TokenFile)
}

func TestNewKubeConfig_TLSServerName(t *testing.T) {
	const (
		host          = "something.test"
		tlsServerName = "something.else.test"
	)

	restConfig := &rest.Config{
		Host: host,
	}
	kubeConfig := NewKubeConfig(restConfig, "")
	assert.Empty(t, kubeConfig.Clusters[host].TLSServerName)

	restConfig = &rest.Config{
		Host: host,
		TLSClientConfig: rest.TLSClientConfig{
			ServerName: tlsServerName,
		},
	}
	kubeConfig = NewKubeConfig(restConfig, "")
	assert.Equal(t, tlsServerName, kubeConfig.Clusters[host].TLSServerName)
}

func TestGetDeploymentReplicas(t *testing.T) {
	manifest := []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.7.9
        ports:
        - containerPort: 80	
`)
	deployment := unstructured.Unstructured{}
	err := yaml.Unmarshal(manifest, &deployment)
	require.NoError(t, err)
	assert.Equal(t, int64(2), *GetDeploymentReplicas(&deployment))
}

func TestGetNilDeploymentReplicas(t *testing.T) {
	manifest := []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: my-pod
spec:
  containers:
  - image: nginx:1.7.9
    name: nginx
    resources:
      requests:
        cpu: 0.2
`)
	noDeployment := unstructured.Unstructured{}
	err := yaml.Unmarshal(manifest, &noDeployment)
	require.NoError(t, err)
	assert.Nil(t, GetDeploymentReplicas(&noDeployment))
}

func TestSplitYAML_SingleObject(t *testing.T) {
	objs, err := SplitYAML([]byte(depWithLabel))
	require.NoError(t, err)
	assert.Len(t, objs, 1)
}

func TestSplitYAML_MultipleObjects(t *testing.T) {
	objs, err := SplitYAML([]byte(depWithLabel + "\n---\n" + depWithLabel))
	require.NoError(t, err)
	assert.Len(t, objs, 2)
}

func TestSplitYAML_TrailingNewLines(t *testing.T) {
	objs, err := SplitYAML([]byte("\n\n\n---" + depWithLabel))
	require.NoError(t, err)
	assert.Len(t, objs, 1)
}

func TestServerResourceGroupForGroupVersionKind(t *testing.T) {
	fakeDisco := &fakedisco.FakeDiscovery{Fake: &testcore.Fake{}}
	fakeDisco.Resources = append(make([]*v1.APIResourceList, 0),
		&v1.APIResourceList{
			GroupVersion: "test.argoproj.io/v1alpha1",
			APIResources: []v1.APIResource{
				{Kind: "TestAllVerbs", Group: "test.argoproj.io", Version: "v1alpha1", Namespaced: true, Verbs: standardVerbs},
				{Kind: "TestSomeVerbs", Group: "test.argoproj.io", Version: "v1alpha1", Namespaced: true, Verbs: []string{"get", "list"}},
			},
		})

	t.Run("Successfully resolve for all verbs", func(t *testing.T) {
		for _, v := range standardVerbs {
			_, err := ServerResourceForGroupVersionKind(fakeDisco, schema.FromAPIVersionAndKind("test.argoproj.io/v1alpha1", "TestAllVerbs"), v)
			assert.NoError(t, err, "Could not resolve verb %s", v)
		}
	})
	t.Run("Successfully resolve for some verbs", func(t *testing.T) {
		for _, v := range []string{"get", "list"} {
			_, err := ServerResourceForGroupVersionKind(fakeDisco, schema.FromAPIVersionAndKind("test.argoproj.io/v1alpha1", "TestSomeVerbs"), v)
			assert.NoError(t, err, "Could not resolve verb %s", v)
		}
	})
	t.Run("Verb not supported", func(t *testing.T) {
		for _, v := range []string{"patch"} {
			_, err := ServerResourceForGroupVersionKind(fakeDisco, schema.FromAPIVersionAndKind("test.argoproj.io/v1alpha1", "TestSomeVerbs"), v)
			assert.Equal(t, err, apierr.NewMethodNotSupported(schema.GroupResource{Group: "test.argoproj.io", Resource: "TestSomeVerbs"}, v))
		}
	})
	t.Run("Resource not found", func(t *testing.T) {
		for _, v := range standardVerbs {
			_, err := ServerResourceForGroupVersionKind(fakeDisco, schema.FromAPIVersionAndKind("test.argoproj.io/v1alpha1", "TestNonExisting"), v)
			assert.Equal(t, err, apierr.NewNotFound(schema.GroupResource{Group: "test.argoproj.io", Resource: "TestNonExisting"}, ""))
		}
	})
}
