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

# Reverse-Proxy Extensions support for Argo CD

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

Argo CD currently supports the creation of [UI extensions][1] allowing
developers to define the visual content of the "more" tab inside
a specific resource. Developers are able to access the resource state to
build the UI. However, currently it isn't possible to use a backend
service to provide additional functionality to extensions. This proposal
defines a new reverse proxy feature in Argo CD, allowing developers to
create a backend service that can be used in UI extensions. Extensions
backend code will live outside Argo CD main repository.

## Motivation

The initiative to implement the anomaly detection capability in Argo CD
highlighted the need to improve the existing UI extensions feature. The
new capability will required the UI to have access to data that isn't
available as part of Application's owned resources. It is necessary to
access an API defined by the extension's development team so the proper
information can be displayed.

## Goals

The following goals are desired but not necessarily all must be
implemented in a given Argo CD release:

#### [G-1] Argo CD (API Server) must have low performance impact when running extensions

Argo CD API server is a critical component as it serves all APIs used by
the CLI as well as the UI. The Argo CD team has no controll over what is
going to be executed in extension's backend service. Thus it is important
that the reverse proxy implementation to cause the lowest possible impact
in the API server while processing high latency requests.

Possible solutions:
- Implement a rate limit layer to protect Argo CD API server
- Implement configurable different types of timeouts (idle connection,
  duration, etc) between Argo CD API server and backend services.
- Implement the reverse proxy as a separate server/pod (needs discussion).

----

#### [G-2] Argo CD admins should be able to define rbacs to define which users can invoke specific extensions

Argo CD Admins must be able to define which extensions are allowed to be
executed by the logged in user. This should be fine grained by Argo CD
project like the current rbac implementation.

----

#### [G-3] Argo CD deployment should be independent from backend services

Extension developers should be able to deploy their backend services
independently from Argo CD. An extension can evolve their internal API and
deploying a new version shouldn't require Argo CD to be updated or
restarted.

----

#### [G-4] Enhance the current Extensions framework to configure backend services

*Not in the first release*

[Argo CD extensions][2] is an `argoproj-labs` project that supports loading
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
extension version. This would need to be considered if Argo CD extensions
eventually get traction.

----

#### [G-5] Setup multiple backend services for the same extension

In case of one Argo CD instance managing applications in multiple clusters, it
will be necessary to configure backend service URLs per cluster for the same
extension. This should be an optional configuration. If only one URL is
configured, that one should be used for all clusters.

----

#### [G-6] Provide safe communication channel between Argo CD API server and extension backend

Argo CD API server should provide configuration for establishing a safe communication
channel with the extension backend. This can be achieved similarly to how Kubernetes
API Server does to [authenticate with aggregated servers][5] by using certificates.

## Non-Goals

It isn't in the scope of this proposal to specify commands in the Argo CD
CLI. This proposal covers the reverse-proxy extension spec that will be
used by Argo CD UI.

## Proposal

### Use cases

The following use cases should be implemented for the conclusion of this
proposal:

#### [UC-1]: As an Argo CD admin, I want to configure a backend services so it can be used by my UI extension

Define a new section in the Argo CD configmap ([argocd-cm.yaml][4])
allowing admins to register and configure new extensions. All enabled
extensions backend will be available to be invoked by the Argo CD UI under
the following API base path:

`<argocd-host>/api/v1/extensions/<extension-name>`

With the configuration below, the expected behavior is explained in the
following examples:

```yaml
extension.config: |
  extensions:
    - name: some-extension
      enabled: true
      backend:
        idleConnTimeout: 10s
        services:
          - url: http://extension-name.com:8080
```

- **Example 1**:

Argo CD API server acts as a reverse-proxy forwarding http requests as
follows:

```
   ┌────────────┐
   │ Argo CD UI │
   └──────┬─────┘
          │
          │ GET http://argo.com/api/v1/extensions/some-extension
          │
          ▼
 ┌──────────────────┐
 │Argo CD API Server│
 └────────┬─────────┘
          │
          │ GET http://extension-name.com:8080
          │
          ▼
  ┌───────────────┐
  │Backend Service│
  └───────────────┘
```

- **Example 2**:

If a backend provides an API under the `/apiv1/metrics` endpoint, Argo CD
should be able to invoke it such as:

```
   ┌────────────┐
   │ Argo CD UI │
   └──────┬─────┘
          │
          │ GET http://argo.com/api/v1/extensions/some-extension/apiv1/metrics/123
          │
          ▼
 ┌──────────────────┐
 │Argo CD API Server│
 └────────┬─────────┘
          │
          │ GET http://extension-name.com:8080/apiv1/metrics/123
          │
          ▼
  ┌───────────────┐
  │Backend Service│
  └───────────────┘
```

- **Example 3**:

In this use-case we have one Argo CD instance connected with different
clusters. There is a requirement defining that every extension instance
needs to be deployed in each of the target clusters. To address this
use-case there is a need to configure multiple backend URLs for the
same extension (one for each cluster). For doing so, the following
configuration should be possible:

```yaml
extension.config: |
  extensions:
    - name: some-extension
      enabled: true
      backend:
        idleConnTimeout: 10s
        services:
          - url: http://extension-name.com:8080
            clusterName: kubernetes.local
          - url: https://extension-name.ppd.cluster.k8s.local:8080
            clusterName: admins@ppd.cluster.k8s.local
```

Note that there is an URL configuration per cluster name. The cluster
name is extracted from the Argo CD cluster secret and must match the
field `data.name`. In this case the UI must send the header
`Argocd-Application-Name` with the full qualified application name
(`<namespace>/<application-name>`).

Example:

`Argocd-Application-Name: preprod/some-application`

With this information, API Server can check in which cluster it should
get the backend URL from. This will be done by inspecting the
Application destination configuration to find the proper cluster name.

The diagram below shows how Argo CD UI could send the request with
the additional header to get the proxy forwarding it to the proper
cluster:

```
   ┌────────────┐
   │ Argo CD UI │
   └──────┬─────┘
          │
          │ GET http://argo.com/api/v1/extensions/some-extension
          │ HEADER: "Argocd-Application-Name: default/ppd-application"
          │
          ▼
 ┌──────────────────┐
 │Argo CD API Server│
 └────────┬─────────┘
          │
          │ GET https://extension-name.ppd.cluster.k8s.local:8080
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
- Scheme, http verb and request body are forwarded as it is
  received by the API server to the backend service.
- Headers will be filtered and not forwarded as it is received in Argo CD
  API server. Sensitive headers will be removed (e.g. `Cookie`).
- A new header is added in the forwared request (`X-Forwarded-Host`) to
  allow ssl redirection.
- This proposal doesn't specify how backends should implement authz or
  authn. This topic could be discussed as a future enhancement to the
  proxy extension feature in Argo CD.

----

#### [UC-2]: As an Argo CD admin, I want to define extensions rbacs so access permissions can be enforced

Extend Argo CD rbac registering a new `ResourceType` for extensions in the
[policy configuration][3]. The current policy permission configuration is
defined as:

```
p, <subject>, <resource>, <action>, <object>, <access>
```

With a new resource type for extensions, admins will be able to configure
access rights per extension per project.

* **Basic config suggestion:**

This is the basic suggestion where admins will be able to define permissions
per project and per extension. In this case namespace specific permissions
isn't covered.

The `object` field must contain the project name and the extension name in
the format `<project>/<extension>`

- *Example 1*:

```
p, role:allow-extensions, extensions, *, some-project/some-extension, allow
```

In the example 1, a permission is configured to allowing the subject
`role:allow-extensions`, for the resource type `extensions`, for all (`*`)
actions, in the project `some-project`, for the extension name
`some-extension`.


- *Example 2*:

```
p, role:allow-extensions, extensions, *, */some-extension, allow
```

In the example 2, the permission is similar to the example 1 with the
difference that the extension `some-extension` will be allowed for all
projects.

- *Example 3*:

```
p, role:allow-extensions, extensions, *, */*, allow
```

In the example 3, the subject `role:allow-extensions` is allowed to
execute extensions in all projects.

* **Advanced config suggestions:**

With advanced RBAC configuration suggestions, admins will be able to define
permissions per project, per namespace and per extension.

There are 3 main approaches to achieve this type of RBAC configuration:

1. `<object>` has addional section for namespace:
```
p, dev, extensions, *, some-project/some-namespace/some-extension, allow
```

2. `<action>` has 2 sections for extension name and namespace:
```
p, dev, extensions, some-extension/some-namespace, some-project/some-application, allow
```

3. `<resource>` has 2 sections for extension type and extension name:
```
p, dev, extensions/some-extension, *, some-project/some-application, allow
```

Reference: [Original discussion][6]

The final RBAC format must be defined and properly documented during implementation.

### Security Considerations

- Argo CD API Server must apply **authn** and **authz** for all incoming
  extensions requests
- Argo CD must authorize requests coming from UI and check that the
  authenticated user has access to invoke a specific URL belonging to an
  extension

### Risks and Mitigations

### Upgrade / Downgrade

## Drawbacks

- Slight increase in Argo CD code base complexity.
- Increased security risk.
- Impact of extensions on overall Argo CD performance (mitigated by rate limiting + idle conn timeout).

## Open Questions

1. What are the possible actions that can be provided to extensions RBAC?
A. This proposal does not define additional RBAC actions for extensions.
Currently the only possible value is `*` which will allow admins to enable
or disable certain extensions per project. If there is a new requirement
to support additional actions for extensions to limit just specific HTTP
verbs for example, an enhancement can be created to extend this
functionality. If this requirement becomes necessary, it won't be a
breaking change as it will be more restrictive.

[1]: https://argo-cd.readthedocs.io/en/stable/developer-guide/ui-extensions/
[2]: https://github.com/argoproj-labs/argocd-extensions
[3]: https://github.com/argoproj/argo-cd/blob/a23bfc3acaa464cbdeafdbbe66d05a121d5d1fb3/server/rbacpolicy/rbacpolicy.go#L17-L25
[4]: https://argo-cd.readthedocs.io/en/stable/operator-manual/argocd-cm.yaml
[5]: https://kubernetes.io/docs/tasks/extend-kubernetes/configure-aggregation-layer/#authentication-flow
[6]: https://github.com/argoproj/argo-cd/pull/10435#discussion_r986941880
