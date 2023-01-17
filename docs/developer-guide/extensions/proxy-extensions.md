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
  namespace: argocd
data:
  extension.config: |
    extensions:
    - name: httpbin-extension
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

- `extensions` (list): defines configurations for all extensions
  enabled.

- `extensions.name` (string): defines the endpoint that will be used to
  register the extension route. Mandatory field.

- `extensions.backend.connectionTimeout` (duration string): is the
  maximum amount of time a dial to the extension server will wait for
  a connect to complete. Optional field. Default: 2 seconds.

- `extensions.backend.keepAlive` (duration string): specifies the
  interval between keep-alive probes for an active network connection
  between the API server and the extension server. Optional field.
  Default: 15 seconds

- `extensions.backend.idleConnectionTimeout` (duration string): is the
  maximum amount of time an idle (keep-alive) connection between the
  API server and the extension server will remain idle before closing
  itself. Optional field. Default: 60 seconds.

- `extensions.backend.maxIdleConnections` (int): controls the maximum
  number of idle (keep-alive) connections between the API server and
  the extension server. Optional field. Default: 30.

- `extensions.backend.services` (list): defines a list with backend url
  by cluster.

- `extensions.backend.services.url` (string): is the address where the
  extension backend must be available. Mandatory field.

- `extensions.backend.services.cluster` (string): Cluster if provided,
  will have to match the application destination name to have requests
  properly forwarded to this service URL. Optional field.


[1]: https://github.com/argoproj/argoproj/blob/master/community/feature-status.md
[2]: https://argo-cd.readthedocs.io/en/stable/operator-manual/argocd-cm.yaml
