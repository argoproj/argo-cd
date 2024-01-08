---
title: Support for Post Renderers
authors:
  - "@surajnarwade"
sponsors:
  - "@anandf"
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2024-01-04
last-updated: 2024-01-04
---

# Neat Enhancement Idea

Support for Post Renderers

## Summary

This proposal suggests the integration of Kustomize based Helm post renderer support into Argo CD, a feature that will allow users to modify Helm chart manifests at the time of rendering. This enhancement aims to provide greater flexibility in customizing deployments, thereby extending Argo CD's utility in diverse environments.

## Motivation

Helm has a support of post renderers which allows users to manipulate, configure rendered manifest before they are installed. Refer [official documentation](https://helm.sh/docs/topics/advanced/#post-rendering) for more.

[FluxCD](https://fluxcd.io/) already supports the Kustomize based [Helm post renderers](https://fluxcd.io/flux/components/helm/helmreleases/#post-renderers).

This proposal is motivated by fluxCD work and following Github issues on the Argo CD.

### Relavant issues

* https://github.com/argoproj/argo-cd/issues/3698
* https://github.com/argoproj/argo-cd/issues/7623

### Goals

* Enhance Flexibility: Enable users to apply custom transformations to Helm chart manifests during the deployment process.
* Increase Compatibility: Broaden the range of Helm charts that can be used with Argo CD, accommodating more complex and specific use cases.
* Improve User Experience: Provide a more seamless and integrated approach to deploying applications using Argo CD and Helm, enhancing the overall user experience.
* Encourage Innovation: By allowing more customization, the feature can stimulate more creative and effective deployment strategies within the Argo CD community.

## Proposal

we can add postRenderers field under the Application spec which is very similar solution as fluxCD

```
  ...
  source:
    ...
    ...
    postRenderers:
    - kustomize:
        patches:
          - target:
              kind: Deployment
            patch: |-
              - op: replace
                path: /spec/template/spec/containers/0/ports/0/containerPort
                value: 443
    ...
```

`postRenderers` will be a list of postRenderers. 

This proposal focuses only on kustomize but in future we can add different postRenderers as well.
Hence we are having `kustomize` struct.
this struct consists of list of patches. this is similar to the one we are using in Kustomize: https://argo-cd.readthedocs.io/en/stable/user-guide/kustomize/ 


as per helm support,

```
helm install mychart stable/wordpress --post-renderer ./path/to/executable
```

executables expects STDIN and outputs STDOUT

as per official example https://github.com/thomastaylor312/advanced-helm-demos/tree/master/post-render, kustomize expects input in the file and then outputs on STDOUT

As supported by helm, we can't exactly use the post-renderer flag here but workaround a shown in the example.

here's the proposed solution:
* if there's post renderer, dump helm template output in the file
* run kustomize build on the file
* output will be on STDOUT as intended

TODO: link POC PR for the same.

### Use cases

#### Use case 1

As a user, I would like to manipulate helm chart to accomodate my needs.

for example, check following taken from the [issue](https://github.com/argoproj/argo-cd/issues/3698),

```
I'm keen on leveraging this feature to tweak some Helm charts that don't fully meet my needs. Take the stable/jenkins chart, for instance; it lacks the option to append extra paths to its ingress object. With post-renderer capabilities, I can easily patch the ingress using kustomize, adding functionalities like a port 80 to 443 redirect for the alb ingress controller. This would really open up more possibilities for customization.
```


### Implementation Details/Notes/Constraints [optional]

* When we install application using ArgoCD, repo server performs the `helm template` command.
* We will save the helm-output into the file.
* then we output kustomize patch into the file.
* finally we run the kustomize build command to output the rendered manifest.

This is potentially equivalent of the script and the post-renderer flag shown in the example:  https://github.com/thomastaylor312/advanced-helm-demos/tree/master/post-render

TODO: link POC PR for the same.


### Detailed examples

```
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: helm-guestbook
  namespace: argocd
  finalizers:
  - resources-finalizer.argocd.argoproj.io
spec:
  destination:
    namespace: helm-guestbook
    server: https://kubernetes.default.svc
  project: default
  source:
    path: helm-guestbook
    repoURL: https://github.com/argoproj/argocd-example-apps
    targetRevision: HEAD
    helm: 
      releaseName: guestbook
    postRenderers:
    - kustomize:
        patches:
          - target:
              kind: Deployment
            patch: |-
              - op: replace
                path: /spec/template/spec/containers/0/ports/0/containerPort
                value: 443
```
