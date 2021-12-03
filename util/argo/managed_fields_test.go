package argo_test

import (
	"testing"

	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNormalize(t *testing.T) {
	t.Run("will normalize desired state respecting manager", func(t *testing.T) {
		// given
		desiredState := StrToUnstructured(DesiredDeployment)
		liveState := StrToUnstructured(LiveDeployment)

		// when
		err := argo.Normalize(liveState, desiredState, []string{"kube-controller-manager"})

		// then
		assert.NoError(t, err)
		desiredReplicas, ok, err := unstructured.NestedFloat64(desiredState.Object, "spec", "replicas")
		assert.False(t, ok)
		assert.NoError(t, err)
		liveReplicas, ok, err := unstructured.NestedFloat64(liveState.Object, "spec", "replicas")
		assert.False(t, ok)
		assert.NoError(t, err)
		assert.Equal(t, liveReplicas, desiredReplicas)
	})
}

func StrToUnstructured(jsonStr string) *unstructured.Unstructured {
	obj := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(jsonStr), &obj)
	if err != nil {
		panic(err)
	}
	return &unstructured.Unstructured{Object: obj}
}

const (
	LiveDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    deployment.kubernetes.io/revision: "1"
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"apps/v1","kind":"Deployment","metadata":{"annotations":{},"labels":{"app.kubernetes.io/instance":"guestbook"},"name":"kustomize-guestbook-ui","namespace":"default"},"spec":{"replicas":1,"revisionHistoryLimit":3,"selector":{"matchLabels":{"app":"guestbook-ui"}},"template":{"metadata":{"labels":{"app":"guestbook-ui"}},"spec":{"containers":[{"image":"gcr.io/heptio-images/ks-guestbook-demo:0.1","name":"guestbook-ui","ports":[{"containerPort":80}]}]}}}}
  creationTimestamp: "2021-12-01T20:36:10Z"
  generation: 4
  labels:
    app.kubernetes.io/instance: guestbook
  managedFields:
  - apiVersion: apps/v1
    fieldsType: FieldsV1
    fieldsV1:
      f:metadata:
        f:annotations:
          .: {}
          f:kubectl.kubernetes.io/last-applied-configuration: {}
        f:labels:
          .: {}
          f:app.kubernetes.io/instance: {}
      f:spec:
        f:progressDeadlineSeconds: {}
        f:replicas: {}
        f:revisionHistoryLimit: {}
        f:selector: {}
        f:strategy:
          f:rollingUpdate:
            .: {}
            f:maxSurge: {}
            f:maxUnavailable: {}
          f:type: {}
        f:template:
          f:metadata:
            f:labels:
              .: {}
              f:app: {}
          f:spec:
            f:containers:
              k:{"name":"guestbook-ui"}:
                .: {}
                f:image: {}
                f:imagePullPolicy: {}
                f:name: {}
                f:ports:
                  .: {}
                  k:{"containerPort":80,"protocol":"TCP"}:
                    .: {}
                    f:containerPort: {}
                    f:protocol: {}
                f:resources: {}
                f:terminationMessagePath: {}
                f:terminationMessagePolicy: {}
            f:dnsPolicy: {}
            f:restartPolicy: {}
            f:schedulerName: {}
            f:securityContext: {}
            f:terminationGracePeriodSeconds: {}
    manager: argocd
    operation: Update
    time: "2021-12-01T20:36:10Z"
  - apiVersion: apps/v1
    fieldsType: FieldsV1
    fieldsV1:
      f:metadata:
        f:annotations:
          f:deployment.kubernetes.io/revision: {}
      f:spec:
        f:replicas: {}
      f:status:
        f:availableReplicas: {}
        f:conditions:
          .: {}
          k:{"type":"Available"}:
            .: {}
            f:lastTransitionTime: {}
            f:lastUpdateTime: {}
            f:message: {}
            f:reason: {}
            f:status: {}
            f:type: {}
          k:{"type":"Progressing"}:
            .: {}
            f:lastTransitionTime: {}
            f:lastUpdateTime: {}
            f:message: {}
            f:reason: {}
            f:status: {}
            f:type: {}
        f:observedGeneration: {}
        f:readyReplicas: {}
        f:replicas: {}
        f:updatedReplicas: {}
    manager: kube-controller-manager
    operation: Update
    time: "2021-12-01T21:07:55Z"
  name: kustomize-guestbook-ui
  namespace: default
  resourceVersion: "5801428"
  uid: f67c43b2-88a8-43aa-ba74-0b50322a5aaf
spec:
  progressDeadlineSeconds: 600
  replicas: 2
  revisionHistoryLimit: 3
  selector:
    matchLabels:
      app: guestbook-ui
  strategy:
    rollingUpdate:
      maxSurge: 25%
      maxUnavailable: 25%
    type: RollingUpdate
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: guestbook-ui
    spec:
      containers:
      - image: gcr.io/heptio-images/ks-guestbook-demo:0.1
        imagePullPolicy: IfNotPresent
        name: guestbook-ui
        ports:
        - containerPort: 80
          protocol: TCP
        resources: {}
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
      dnsPolicy: ClusterFirst
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext: {}
      terminationGracePeriodSeconds: 30
status:
  availableReplicas: 2
  conditions:
  - lastTransitionTime: "2021-12-01T20:36:10Z"
    lastUpdateTime: "2021-12-01T20:36:12Z"
    message: ReplicaSet "kustomize-guestbook-ui-779bc8b498" has successfully progressed.
    reason: NewReplicaSetAvailable
    status: "True"
    type: Progressing
  - lastTransitionTime: "2021-12-01T21:07:55Z"
    lastUpdateTime: "2021-12-01T21:07:55Z"
    message: Deployment has minimum availability.
    reason: MinimumReplicasAvailable
    status: "True"
    type: Available
  observedGeneration: 4
  readyReplicas: 2
  replicas: 2
  updatedReplicas: 2
`

	DesiredDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app.kubernetes.io/instance: guestbook
  name: kustomize-guestbook-ui
  namespace: default
spec:
  replicas: 1
  revisionHistoryLimit: 3
  selector:
    matchLabels:
      app: guestbook-ui
  template:
    metadata:
      labels:
        app: guestbook-ui
    spec:
      containers:
        - image: 'gcr.io/heptio-images/ks-guestbook-demo:0.1'
          name: guestbook-ui
          ports:
            - containerPort: 80
`
)
