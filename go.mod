module github.com/argoproj/gitops-engine

go 1.16

require (
	github.com/davecgh/go-spew v1.1.1
	github.com/evanphx/json-patch v4.12.0+incompatible
	github.com/go-logr/logr v1.2.0
	github.com/golang/mock v1.5.0
	github.com/spf13/cobra v1.2.1
	github.com/stretchr/testify v1.7.0
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	k8s.io/api v0.23.1
	k8s.io/apiextensions-apiserver v0.23.1
	k8s.io/apimachinery v0.23.1
	k8s.io/cli-runtime v0.23.1
	k8s.io/client-go v0.23.1
	k8s.io/klog/v2 v2.30.0
	k8s.io/kube-aggregator v0.23.1
	k8s.io/kubectl v0.23.1
	k8s.io/kubernetes v1.23.1
	sigs.k8s.io/yaml v1.2.0
)

replace (
	// https://github.com/kubernetes/kubernetes/issues/79384#issuecomment-505627280
	k8s.io/api => k8s.io/api v0.23.1
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.23.1 // indirect
	k8s.io/apimachinery => k8s.io/apimachinery v0.23.1 // indirect
	k8s.io/apiserver => k8s.io/apiserver v0.23.1
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.23.1
	k8s.io/client-go => k8s.io/client-go v0.23.1
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.23.1
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.23.1
	k8s.io/code-generator => k8s.io/code-generator v0.23.1
	k8s.io/component-base => k8s.io/component-base v0.23.1
	k8s.io/component-helpers => k8s.io/component-helpers v0.23.1
	k8s.io/controller-manager => k8s.io/controller-manager v0.23.1
	k8s.io/cri-api => k8s.io/cri-api v0.23.1
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.23.1
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.23.1
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.23.1
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.23.1
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.23.1
	k8s.io/kubectl => k8s.io/kubectl v0.23.1
	k8s.io/kubelet => k8s.io/kubelet v0.23.1
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.23.1
	k8s.io/metrics => k8s.io/metrics v0.23.1
	k8s.io/mount-utils => k8s.io/mount-utils v0.23.1
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.23.1
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.23.1
)
