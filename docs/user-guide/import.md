# Importing Argo CD go packages 

## Issue 

When importing Argo CD packages in your own projects, you may face some errors when downloading the dependencies, such as "unknown revision v0.0.0". This is because Argo CD directly depends on some Kubernetes packages which have these unknown v0.0.0 versions in their go.mod.

## Solution

Add a replace section in your own go.mod as same as the replace section of the corresponding Argo CD version's go.mod. In order to find the go.mod for a specific version, navigate to the [Argo CD repository](https://github.com/argoproj/argo-cd/) and click on the switch branches/tags dropdown to select the version you are looking for. Now you can view the go.mod file for a specific version along with all other files.

## Example

If you are using Argo CD v2.4.15, your go.mod should contain the following:

```
replace (
    // https://github.com/golang/go/issues/33546#issuecomment-519656923
    github.com/go-check/check => github.com/go-check/check v0.0.0-20180628173108-788fd7840127

    github.com/golang/protobuf => github.com/golang/protobuf v1.4.2
    github.com/gorilla/websocket => github.com/gorilla/websocket v1.4.2
    github.com/grpc-ecosystem/grpc-gateway => github.com/grpc-ecosystem/grpc-gateway v1.16.0
    github.com/improbable-eng/grpc-web => github.com/improbable-eng/grpc-web v0.0.0-20181111100011-16092bd1d58a

    // Avoid CVE-2022-28948
    gopkg.in/yaml.v3 => gopkg.in/yaml.v3 v3.0.1

    // https://github.com/kubernetes/kubernetes/issues/79384#issuecomment-505627280
    k8s.io/api => k8s.io/api v0.23.1
    k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.23.1
    k8s.io/apimachinery => k8s.io/apimachinery v0.23.1
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
```
