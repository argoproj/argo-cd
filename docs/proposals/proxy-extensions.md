---
title: Reverse Proxy Extensions

authors:
- "@leoluz"

sponsors:
- TBD

reviewers:
- TBD

approvers:
- TBD

creation-date: 2022-07-23

---

# Reverse-Proxy Extensions support for ArgoCD

Enable UI extensions to use a backend service.

* [Summary](#summary)
* [Motivation](#motivation)
* [Goals](#goals)
* [Non-Goals](#non-goals)
* [Proposal](#proposal)
    * [Use cases](#use-cases)
    * [Security Considerations](#security-considerations)
    * [Risks and Mitigations](#risks-and-mitigations)
    * [Upgrade / Downgrade](#upgrade--downgrade)
* [Drawbacks](#drawbacks)
* [Open Questions](#open-questions)

---

## Summary

ArgoCD currently supports the creation of [UI extensions][1] allowing
developers to define the visual content of the "more" tab inside
a specific resource. Developers are able to access the resource state to
build the UI. However, currently it isn't possible to use a backend
service to provide additional functionality to extensions. This proposal
defines a new reverse proxy feature in ArgoCD, allowing developers to
create a backend service that can be used in UI extensions.

## Motivation

The initiative to implement the anomaly detection capability in ArgoCD
highlighted the need to improve the existing UI extensions feature. The
new capability will required the UI to have access to data that isn't
available as part of Application's owned resources. It is necessary to
access an API defined by the extension's development team so the proper
information can be displayed.

## Goals

The following goals are desired but not necessarily all must be
implemented in a given ArgoCD release. 

----

#### [G-1] ArgoCD (API Server) must have low performance impact when running extensions

ArgoCD API server is a critical component as it serves all APIs used by
the CLI as well as the UI. The ArgoCD team has no controll over what is
going to be executed in extension's backend service. Thus it is important
that the reverse proxy implementation to cause the lowest possible impact
in the API server while processing high latency requests.

Possible solutions:
- reverse proxy implemented as an independent server
- reverse proxy invoke backend services asynchronously

----

#### [G-2] ArgoCD admins should be able to define rbacs to define which users can invoke specific extensions

ArgoCD Admins must be able to define which extensions are allowed to be
executed by the logged in user. This should be fine grained by ArgoCD
project like the current rbac implementation.

----

#### [G-3] ArgoCD deployment should be independent from backend services

Extension developers should be able to deploy their backend services
indepedendtly from ArgoCD. An extension can evolve their internal API and
deploying a new version shouldn't require ArgoCD to be updated or
restarted.

----

#### [G-4] Enhance the current Extensions framework to configure backend services

[ArgoCD extensions][2] is an `argoproj-labs` project that supports loading
extensions in runtime. Currently the project is implementing a controller
that defines and reconciles the custom resource `ArgoCDExtension`. This
CRD should be enhanced to provide the ability to define backend services
that will be used by the extension. Once configured UI can send requests
to API server in a specific endpoint. API server will act as a reverse
proxy receiving the request from the UI and routing to the appropriate
backend service.

Example:
```yaml 
apiVersion: argoproj.io/v1alpha1
kind: ArgoCDExtension
metadata:
  name: my-cool-extention
  finalizers:
    - extensions-finalizer.argocd.argoproj.io
spec:
  sources:
    - git:
        url: https://github.com/some-org/my-cool-extension.git
  backend:
    serviceName: some-backend-svc
    endpoint: /some-backend
```

**Note**: While this is a nice-to-have, it won't be part of the first proxy
extension version. This would need to be considered if ArgoCD extensions
eventually get traction.

## Non-Goals

It isn't in the scope of this proposal to specify commands in the ArgoCD
CLI. This proposal covers the reverse-proxy extension spec that will be
used by ArgoCD UI.

## Proposal

### Use cases

The following use cases should be implemented for the conclusion of this
proposal:

----

#### [UC-1]: As an ArgoCD admin, I want to configure a backend services so it can be used by my UI extension

Define a new section in the ArgoCD configmap ([argocd-cm.yaml][4])
allowing admins to register and configure new extensions. All enabled
extensions backend will be available to be invoked by the ArgoCD UI under
the following API base path:

`<argocd-host>/api/v1/extensions/<extension-name>`


With the configuration bellow, the expected behaviour is explained in the
following examples:

```yaml
extension.config:
| extensions:
    - name: some-extension
      enabled: true
      backend:
        idleConnTimeout: 10s
        url: http://extension-host.com:8080
```

- **Example 1**:

ArgoCD API server acts as a reverse-proxy forwarding http requests as
follows:

```

   ┌────────────┐
   │ ArgoCD UI  │
   └──────┬─────┘
          │
          │ GET http://argo.com/api/v1/extensions/some-extension
          │
          ▼
 ┌─────────────────┐
 │ArgoCD API Server│
 └────────┬────────┘
          │
          │ GET http://extension-host.com:8080
          │
          ▼
  ┌───────────────┐
  │Backend Service│
  └───────────────┘

```

- **Example 2**:

If a backend provides an API under the `/apiv1/metrics` endpoint, ArgoCD
should be able to invoke it such as:

```
   ┌────────────┐
   │ ArgoCD UI  │
   └──────┬─────┘
          │
          │ GET http://argo.com/api/v1/extensions/some-extension/apiv1/metrics/123
          │
          ▼
 ┌─────────────────┐
 │ArgoCD API Server│
 └────────┬────────┘
          │
          │ GET http://extension-host.com:8080/apiv1/metrics/123
          │
          ▼
  ┌───────────────┐
  │Backend Service│
  └───────────────┘
```

##### Considerations

- The `idleConnTimeout` can be used to avoid accumulating too many
  goroutines waiting slow for extensions. In this case a proper timeout
  error (408) should be returned to the browser.
- Headers, scheme, http verb and request body are forwarded as it is
  received by the API server to the backend service.
- A new header is added in the forwared request (`X-Forwarded-Host`) to
  allow ssl redirection

----

#### [UC-2]: As an ArgoCD admin, I want to define extensions rbacs so access permissions can be enforced

Extend ArgoCD rbac registering a new `ResourceType` for extensions in the
[policy configuration][3]. The current policy permission configuration is
defined as:

```
p, <subject>, <resource>, <action>, <object>, <access>
```

With a new resource type for extensions, admins will be able to configure
access rights per extension per project. The `object` field must contain
the project name and the extension name in the format
`<project>/<extension>`

- **Example 1**:

```
p, role:allow-extensions, extensions, *, some-project/some-extension, allow
```

In the example 1, a permission is configured to allowing the subject
`role:allow-extensions`, for the resource type `extensions`, for all (`*`)
actions, in the project `some-project`, for the extension name
`some-extension`.


- **Example 2**:

```
p, role:allow-extensions, extensions, *, */some-extension, allow
```

In the example 2, the permission is similar to the example 1 with the
difference that the extension `some-extension` will be allowed for all
projects.

- **Example 3**:

```
p, role:allow-extensions, extensions, *, */*, allow
```

In the example 3, the subject `role:allow-extensions` is allowed to
execute extensions in all projects.

### Security Considerations

- ArgoCD API Server must apply **authn** and **authz** for all incoming
  extensions requests
- ArgoCD must authorize requests coming from UI and check that the
  authenticated user has access to invoke a specific URL belonging to an
  extension

### Risks and Mitigations

### Upgrade / Downgrade

## Drawbacks

- Slight increase in ArgoCD code base complexity.

## Open Questions

[1]: https://argo-cd.readthedocs.io/en/stable/developer-guide/ui-extensions/
[2]: https://github.com/argoproj-labs/argocd-extensions
[3]: https://github.com/argoproj/argo-cd/blob/a23bfc3acaa464cbdeafdbbe66d05a121d5d1fb3/server/rbacpolicy/rbacpolicy.go#L17-L25
[4]: https://argo-cd.readthedocs.io/en/stable/operator-manual/argocd-cm.yaml
