package kube

import (
	"encoding/json"
	"log"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
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

func TestUnsetLabels(t *testing.T) {
	for _, yamlStr := range []string{depWithLabel} {
		var obj unstructured.Unstructured
		err := yaml.Unmarshal([]byte(yamlStr), &obj)
		assert.Nil(t, err)

		UnsetLabel(&obj, "foo")

		manifestBytes, err := json.MarshalIndent(obj.Object, "", "  ")
		assert.Nil(t, err)
		log.Println(string(manifestBytes))

		var dep extv1beta1.Deployment
		err = json.Unmarshal(manifestBytes, &dep)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(dep.ObjectMeta.Labels))
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
	assert.NoError(t, err)
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
	assert.NoError(t, err)
	assert.Nil(t, GetDeploymentReplicas(&noDeployment))
}
