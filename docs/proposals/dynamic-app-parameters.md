---
title: Dynamic Application Parameters
authors:
  - "@jannfis" # Authors' github accounts here.
sponsors:
  - TBD        # List all interested parties here.
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2023-01-17
last-updated: 2023-01-17
---

# Dynamic application parameters

## Open questions

* Should the value of a dynamic parameter be visible to a user who has `get` permissions of an Application (see also Security Implications below)?

## Summary

This document proposes the introduction of a new type of application parameters which are resolved dynamically at manifest rendering time. Its values will be retrieved from the application's destination cluster. Details of this are described in this proposal.

## Motivation

Helm provides a templating function called `lookup` from within their charts’ templates. Using this function, a chart can be parameterized with values that are retrieved from the live state of the cluster where the Helm chart is going to be installed to. This mechanism can be used for several purposes, such as getting the FQDN of a Route/Ingress object, or using a value that is stored in a ConfigMap object, amongst others and is promoting re-use of a chart with minimum effort required to parameterize it.

In a typical Argo CD installation however, the component that’s used to render Helm charts (the repository server) does not have access to nor contextual knowledge (beside a few informational assets such cluster version and APIs provided) about the cluster where the chart is going to be installed to. Argo CD performs an offline templating of the chart, so there is no way for this kind of dynamic parameterization to work.

The current workaround is to parameterize all Argo CD Applications using explicit, application specific parameters. This can become tedious and cumbersome, especially when using mechanisms such as ApplicationSet to render large amounts of Applications. Also, secret data is not to be stored in Application manifests, since these objects are considered non-confidential.

The community wants to leverage the functionality behind `helm lookup` with Argo CD. However natively supporting `helm lookup` would require a significant architectural change, and it would also have severe security implications to be resolved.

## Non-goals

* Supporting `helm lookup` “as is” is a non-goal. Example, `helm lookup` constructs like `“lookup "v1" "Pod" "mynamespace" "mypod"` are not supported. This proposal addresses an Argo CD-native mechanism to support consumption of in-cluster information in helm charts and possibly other types of Applications in the future. 
* Emulate or otherwise make this feature a replacement for secrets management (albeit it may allow Applications to read values stored in secrets on the cluster)

## Proposal

The authors of this document propose the introduction of a new kind of parameter for Helm applications (and later possibly other types of Applications): The dynamic parameter.

Currently, Helm Applications within Argo CD support _statically defined_ parameters, which are simple key/value mappings and which need to be set to the desired value before the Application's manifests are being rendered.

A dynamic parameter is defined similarly to how a static parameter is defined currently, but instead of supplying a static value, it is configured using a reference to a resource that exists in the destination cluster. 

For example:

```yaml
kind: Application
apiVersion: argoproj.io/v1alpha1
metadata:
  name: foo-app
spec:
  project: default
  source:
    repoURL: https://some.helm.repo.com
    chart: foo
    targetRevision: 1.0.x
    helm:
      parameters:
      - name: foo
        value: bar
      dynamicParameters:
      - name: bar
        forceString: true # optional, will use --set-string
        resourceRef:
          kind: ConfigMap
          group: ""
          name: some-cm
          namespace: some-ns # if omitted, use target namespace
          path: .data.somekey # if omitted, set param to true if resource exists or to false if resource does not exist
  destination:
    server: https://kubernetes.default.svc
    namespace: foo-app
```
	
The above example defines a Helm chart that is going to be installed to the local cluster where Argo CD is running on, into the namespace foo-app.

The chart will be rendered using two parameters that are set in the Application spec:

* A parameter `foo` with a static value of `bar`
* A parameter `bar`, whose value will be determined at manifest generation time

Defining the static parameter `foo` is an existing, well-known and documented way of parameterization. However, the dynamic parameter `bar` is the subject of this proposal.

As opposed to a static parameter, a dynamic parameter’s value isn’t known at the time of Application creation and can change over the course of time. The value will be gathered by the application controller from the cluster (or more specifically, the cluster cache) at the time the manifests are rendered, and then provided to helm template command line arguments through the repository server’s gRPC API when manifest generation is requested.

## Usage and use-case

While this approach does not make the `lookup` function of Helm work, it would provide an alternative way to use values that exist in clusters for rendering a chart. Any sane Helm chart should not rely solely on lookup, but most likely will provide a way to either statically parameterize a setting or use lookup if no parameterization is given (because `lookup` is neither supported for `helm template` runs nor any `–dry-run` operation). 

For example, consider the following template snippet that would use the value of `some.param`, and if that parameter is not explicitly set, will use the `lookup` function instead:

```
{{ some.param | default (lookup "v1" "ConfigMap" "guestbook" "some-cm").data.some-field }}
```

The parameter `some.param` could be set dynamically in the `Application` spec as follows, thereby effectively emulating the lookup:

```yaml
dynamicParameters:
- name: some.param
  resourceRef:
    kind: ConfigMap
    group: ""
    name: some-cm
    namespace: guestbook
    path: .data.some-field
```

## Resource references

The resource to be referenced is identified by the combination of its `group`, `kind`, `name` and optionally `namespace` along with the Application’s destination cluster as specified in `.spec.destination.server`. If `namespace` is omitted, and the resource’s kind is namespace-scoped, then the application's target namespace will be used to lookup the resource.

## Value determination semantics

If `path` is given, the lookup will retrieve the value from the resource specified by path. The path is interpreted as a JSON path expression, similar to the format expected by `kubectl get -o jsonpath`. If `path` is not given, the parameter’s value would be either set to `true` (if the resource exists) or to `false` (if the resource does not exist).

## Access control

Access control to the resource the value will be governed by the restrictions set for the Application in the corresponding AppProject. If the AppProject does not allow access to the referenced resource on the destination cluster, then defining a dynamic parameter referring to a forbidden resource will throw an error during manifest generation (see below).

A new field in the AppProject should be considered. Existing `resourceAllowList` and `resourceDenyList` fields define both, read and write access to certain resources or resource classes. However, 

## Value determination interval

As noted above, the value for a dynamic parameter will be determined at manifest rendering time (i.e. at the time when the helm template command is run by Argo CD). This means that a change to a referenced resource will not trigger an immediate re-rendering of the source chart. The application needs to be either refreshed manually, or its freshness must expire.

## Cluster access and watches

The value for a dynamic parameter will be taken from the cluster cache, and there will be no additional calls to the remote cluster API. In the first incarnation of this feature, no additional watches will be established, nor will there be any new hooks for detecting modifications to the cluster cache.

## Error handling

Generally, failure to resolve a dynamic parameter should lead to an error in manifest generation, preventing a sync. The following error situations, additional to generic error situations, could occur when resolving dynamic parameters:

### Access is forbidden by the AppProject

If a referenced resource is not allowed to be accessed through the AppProject, manifest generation would terminate with an error stating that access to the resource is forbidden by configuration, without disclosing any other error details to the user. Details can be logged instead.

### Access is forbidden by Kubernetes

When access to the resource is forbidden by Kubernetes, the resource will not exist in the cluster cache. In this case, a generic “Resource not found” error should be returned, without disclosing any other error details to the user. Details can be logged instead.

### Unknown kind or group

When the user references a kind or group that does not exist on the destination cluster (possibly a typo in the configuration, etc), a detailed error message may be displayed to the user, since it is assumed that the user would have access to the resource.

### Referenced resource does not exist

When the referenced resource does not exist, and path is not set, it will set the parameter’s value to false. If a path is given, an error is returned to the user that the referenced resource does not exist.

### Field in referenced resource does not exist

When there is a field reference given as path, but this path cannot be found in the referenced resource, a specific error message (e.g. path `.data.foo` does not exist) may be returned to the user, since it is assumed that the user has access to the resource.

## Security implications

### Preventing reading arbitrary cluster resources

Dynamic parameters must not be allowed to read arbitrary resources on the cluster. Instead, there must be some access control mechanisms.

Access control for retrieving values from clusters will be enforced by restrictions in the `AppProject`, some of which already are in place, and will be using the following principles:

* If access to the resource would be allowed through existing `namespaceResourceWhitelist`, `namespaceResourceBlacklist`, `clusterResourceWhitelist` and `clusterResourceBlacklist` configuration, then a dynamic parameter may access the resource as well.
* Additionally, new access controls specifically for allowing read-only access to dynamic parameters should be introduced. In addition to referencing accessable resources through `kind`, `group` and (possibly) `namespace` attributes, it should optionally be more refinable by a specific resource `name`. The fields should be called `namespaceReadOnlyAllowlist` and `clusterReadOnlyAllowList` respectively, and not be implicitly bound by the field name to the dynamic parameter feature for future applications. There's no need (yet) for a deny list in this scenario.

### Transparency of dynamic parameters' values

It should be either documented that dynamic parameters should not refer to confidential data stored in a cluster (e.g. a Secret), or we need to take care not to leak any of the values back to a user who has e.g. `get` or `sync` access for the Application. Leakage could occur anywhere, in error messages, logs, etc.

We should keep in mind that even with dynamic parameters, the values will be part of the command line when `helm template` is executed.

Therefore, the recommendation is to make it clear to not use this feature for confidential items.

## Alternatives

None considered so far.