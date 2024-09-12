# Proxy Extensions
*Current Status: [Alpha][1] (Since v2.7.0)*

## Overview

With UI extensions it is possible to enhance Argo CD web interface to
provide valuable data to the user. However the data is restricted to
the resources that belongs to the Application. With proxy extensions
it is also possible to add additional functionality that have access
to data provided by backend services. In this case Argo CD API server
acts as a reverse-proxy authenticating and authorizing incoming
requests before forwarding to the backend service.

## Configuration

As proxy extension is in [Alpha][1] phase, the feature is disabled by
default. To enable it, it is necessary to configure the feature flag
in Argo CD command parameters. The easiest way to properly enable
this feature flag is by adding the `server.enable.proxy.extension` key
in the existing `argocd-cmd-params-cm`. For example:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cmd-params-cm
  namespace: argocd
data:
  server.enable.proxy.extension: "true"
```

Once the proxy extension is enabled, it can be configured in the main
Argo CD configmap ([argocd-cm][2]).

The example below demonstrates all possible configurations available
for proxy extensions:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
data:
  extension.config: |
    extensions:
    - name: httpbin
      backend:
        connectionTimeout: 2s
        keepAlive: 15s
        idleConnectionTimeout: 60s
        maxIdleConnections: 30
        services:
        - url: http://httpbin.org
          headers:
          - name: some-header
            value: '$some.argocd.secret.key'
          cluster:
            name: some-cluster
            server: https://some-cluster
```

Note: There is no need to restart Argo CD Server after modifiying the
`extension.config` entry in Argo CD configmap. Changes will be
automatically applied. A new proxy registry will be built making
all new incoming extensions requests (`<argocd-host>/extensions/*`) to
respect the new configuration.

Every configuration entry is explained below:

#### `extensions` (*list*)

Defines configurations for all extensions enabled.

#### `extensions.name` (*string*)
(mandatory)

Defines the endpoint that will be used to register the extension
route. For example, if the value of the property is `extensions.name:
my-extension` then the backend service will be exposed under the
following url:

    <argocd-host>/extensions/my-extension

#### `extensions.backend.connectionTimeout` (*duration string*)
(optional. Default: 2s)

Is the maximum amount of time a dial to the extension server will wait
for a connect to complete. 

#### `extensions.backend.keepAlive` (*duration string*)
(optional. Default: 15s)

Specifies the interval between keep-alive probes for an active network
connection between the API server and the extension server.

#### `extensions.backend.idleConnectionTimeout` (*duration string*)
(optional. Default: 60s)

Is the maximum amount of time an idle (keep-alive) connection between
the API server and the extension server will remain idle before
closing itself.

#### `extensions.backend.maxIdleConnections` (*int*)
(optional. Default: 30)

Controls the maximum number of idle (keep-alive) connections between
the API server and the extension server.

#### `extensions.backend.services` (*list*)

Defines a list with backend url by cluster.

#### `extensions.backend.services.url` (*string*)
(mandatory)

Is the address where the extension backend must be available.

#### `extensions.backend.services.headers` (*list*)

If provided, the headers list will be added on all outgoing requests
for this service config. Existing headers in the incoming request with
the same name will be overridden by the one in this list. Reserved header
names will be ignored (see the [headers](#incoming-request-headers) below).

#### `extensions.backend.services.headers.name` (*string*)
(mandatory)

Defines the name of the header. It is a mandatory field if a header is
provided.

#### `extensions.backend.services.headers.value` (*string*)
(mandatory)

Defines the value of the header. It is a mandatory field if a header is
provided. The value can be provided as verbatim or as a reference to an
Argo CD secret key. In order to provide it as a reference, it is
necessary to prefix it with a dollar sign.

Example:

    value: '$some.argocd.secret.key'

In the example above, the value will be replaced with the one from
the argocd-secret with key 'some.argocd.secret.key'.

#### `extensions.backend.services.cluster` (*object*)
(optional)

If provided, and multiple services are configured, will have to match
the application destination name or server to have requests properly
forwarded to this service URL. If there are multiple backends for the
same extension this field is required. In this case at least one of
the two will be required: name or server. It is better to provide both
values to avoid problems with applications unable to send requests to
the proper backend service. If only one backend service is
configured, this field is ignored, and all requests are forwarded to
the configured one.

#### `extensions.backend.services.cluster.name` (*string*)
(optional)

It will be matched with the value from
`Application.Spec.Destination.Name`

#### `extensions.backend.services.cluster.server` (*string*)
(optional)

It will be matched with the value from
`Application.Spec.Destination.Server`. 

## Usage

Once a proxy extension is configured it will be made available under
the `/extensions/<extension-name>` endpoint exposed by Argo CD API
server. The example above will proxy requests to
`<apiserver-host>/extensions/httpbin/` to `http://httpbin.org`.

The diagram below illustrates an interaction possible with this
configuration:

```
                                              ┌─────────────┐
                                              │ Argo CD UI  │
                                              └────┬────────┘
                                                   │  ▲
  GET <apiserver-host>/extensions/httpbin/anything │  │ 200 OK
            + authn/authz headers                  │  │
                                                   ▼  │
                                            ┌─────────┴────────┐
                                            │Argo CD API Server│
                                            └──────┬───────────┘
                                                   │  ▲
                   GET http://httpbin.org/anything │  │ 200 OK
                                                   │  │
                                                   ▼  │
                                             ┌────────┴────────┐
                                             │ Backend Service │
                                             └─────────────────┘
```

### Incoming Request Headers

Note that Argo CD API Server requires additional HTTP headers to be
sent in order to enforce if the incoming request is authenticated and
authorized before being proxied to the backend service. The headers
are documented below:

#### `Cookie`

Argo CD UI keeps the authentication token stored in a cookie
(`argocd.token`). This value needs to be sent in the `Cookie` header
so the API server can validate its authenticity.

Example: 

    Cookie: argocd.token=eyJhbGciOiJIUzI1Ni...

The entire Argo CD cookie list can also be sent. The API server will
only use the `argocd.token` attribute in this case.

#### `Argocd-Application-Name` (mandatory)

This is the name of the project for the application for which the
extension is being invoked. The header value must follow the format:
`"<namespace>:<app-name>"`.

Example:

    Argocd-Application-Name: namespace:app-name

#### `Argocd-Project-Name` (mandatory)

The logged in user must have access to this project in order to be
authorized.

Example:

    Argocd-Project-Name: default

Argo CD API Server will ensure that the logged in user has the
permission to access the resources provided by the headers above. The
validation is based on pre-configured [Argo CD RBAC rules][3]. The
same headers are also sent to the backend service. The backend service
must also validate if the validated headers are compatible with the
rest of the incoming request.

### Outgoing Requests Headers

Requests sent to backend services will be decorated with additional
headers. The outgoing request headers are documented below:

#### `Argocd-Target-Cluster-Name`

Will be populated with the value from `app.Spec.Destination.Name` if
it is not empty string in the application resource.

#### `Argocd-Target-Cluster-URL`

Will be populated with the value from `app.Spec.Destination.Server` if
it is not empty string is the Application resource.

Note that additional pre-configured headers can be added to outgoing
request. See [backend service headers](#extensionsbackendservicesheaders-list)
section for more details.

#### `Argocd-Username`

Will be populated with the username logged in Argo CD.

#### `Argocd-User-Groups`

Will be populated with the 'groups' claim from the user logged in Argo CD.

### Multi Backend Use-Case

In some cases when Argo CD is configured to sync with multiple remote
clusters, there might be a need to call a specific backend service in
each of those clusters. The proxy-extension can be configured to
address this use-case by defining multiple services for the same
extension. Consider the following configuration as an example:

```yaml
extension.config: |
  extensions:
  - name: some-extension
    backend:
      services:
      - url: http://extension-name.com:8080
        cluster
          name: kubernetes.local
      - url: https://extension-name.ppd.cluster.k8s.local:8080
        cluster
          server: user@ppd.cluster.k8s.local
```

In the example above, the API server will inspect the Application
destination to verify which URL should be used to proxy the incoming
request to.

## Security

When a request to `/extensions/*` reaches the API Server, it will
first verify if it is authenticated with a valid token. It does so by
inspecting if the `Cookie` header is properly sent from Argo CD UI
extension.

Once the request is authenticated it is then verified if the
user has permission to invoke this extension. The permission is
enforced by Argo CD RBAC configuration. The details about how to
configure the RBAC for proxy-extensions can be found in the [RBAC
documentation][3] page.

Once the request is authenticated and authorized by the API server, it
is then sanitized before being sent to the backend service. The
request sanitization will remove sensitive information from the
request like the `Cookie` and `Authorization` headers.

A new `Authorization` header can be added to the outgoing request by
defining it as a header in the `extensions.backend.services.headers`
configuration. Consider the following example:

```yaml
extension.config: |
  extensions:
  - name: some-extension
    backend:
      services:
      - url: http://extension-name.com:8080
        headers:
        - name: Authorization
          value: '$some-extension.authorization.header'
```

In the example above, all requests sent to
`http://extension-name.com:8080` will have an additional
`Authorization` header. The value of this header will be the one from
the [argocd-secret](../../operator-manual/argocd-secret-yaml.md) with
key `some-extension.authorization.header`

[1]: https://github.com/argoproj/argoproj/blob/master/community/feature-status.md
[2]: https://argo-cd.readthedocs.io/en/stable/operator-manual/argocd-cm.yaml
[3]: ../../operator-manual/rbac.md#the-extensions-resource
