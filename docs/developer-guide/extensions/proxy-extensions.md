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

Proxy extensions are configured in the main Argo CD configmap
([argocd-cm][2]).

The example below demonstrate all possible configurations available
for proxy extensions:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd data:
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
          cluster: https://some-cluster
```

Every configuration entry is explained below:

#### `extensions` (*list*)

Defines configurations for all extensions enabled.

#### `extensions.name` (*string*)
(mandatory)

Defines the endpoint that will be used to register the extension
route.

#### `extensions.backend.connectionTimeout` (*duration string*)
(optional. Default: 2s)

Is the maximum amount of time a dial to the extension server will wait
for a connect to complete. 

#### `extensions.backend.keepAlive` (*duration string*)
(optional. Default: 15s)

Specifies the interval between keep-alive probes for an active network
connection between the API server and the extension server. Optional
field.

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

#### `extensions.backend.services.cluster` (*string*)
(optional)

If provided, will have to match the application destination name to
have requests properly forwarded to this service URL.

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
                                             │Backend Extension│
                                             └─────────────────┘
```

Note that Argo CD API Server requires additional HTTP headers to be
sent in order to enforce if the incoming request is authenticated and
authorized before being proxied to the Backend Extension. The headers
are documented below:

#### `Cookie` (*mandatory*)

Argo CD UI keeps the authentication token stored in a cookie
(`argocd.token`). This value needs to be sent in the `Cookie` header
so the API server can validate its authenticity.

Example: 

    Cookie: "argocd.token=eyJhbGciOiJIUzI1Ni..."

The entire Argo CD cookie list can also be sent. The API server will
filter out the `argocd.token` automatically in this case.


#### `Argocd-Application-Name` (mandatory)

The logged in user must have read permission in this application name
in order to be authorized. The header value must follow the format:
`"<namespace>:<app-name>"`.

Example:

    Argocd-Application-Name: "namespace:app-name"

#### `Argocd-Project-Name` (mandatory)

The logged in user must have access to this project in order to be
authorized.

Example:

    Argocd-Project-Name: "default"

#### `Argocd-Resource-GVK-Name` (optional)

To provide additional level of validation, the requested resource can
be sent in this header. In this case the API server will validate that
this specific resource belongs to the application identified by the
`Argocd-Application-Name` header.

The value must follow the format:

    "<apiVersion>:<kind>:<metadata.name>"

Example:

    Argocd-Resource-GVK-Name: "apps/v1:Pod:some-pod"

It is also possible to send multiple values in this header separated
by commas.

## Security

When a request to `/extensions/*` reaches the API Server, it will
first verify if it is authenticated with a valid token. It does so by
inspecting if the `Cookie` header is properly sent from Argo CD UI
extension. Once the request is authenticated it is then verified if the
user has permission to invoke this extension. 

[1]: https://github.com/argoproj/argoproj/blob/master/community/feature-status.md
[2]: https://argo-cd.readthedocs.io/en/stable/operator-manual/argocd-cm.yaml
