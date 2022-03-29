# Extensions

An extension is a way to add new capabilities to the UI and API.

Use cases include:

* Surfacing high-level application telemetry and insights.
* Recommending Kubernetes best practices.
* Alerting users about vulnerabilities in their application.

Extensions consist to one or two components:

* A JavaScript module that exports UI extensions.
* Optionally, an API extension that allows UI requests to be proxied to another service.

Only install extensions from trusted sources.

## UI Extension

A Javascript module should be:

* Packaged into a single `.js` file.
* Install on each Argo CD server in `/tmp/extensions/{name}.js`
* Listed in `/tmp/extensions/index.json`.

You can add the following types of extension:

* `appToolbarButton` - A button added to the application toolbar.
* `appPanel` - A sliding panel.
* `appStatusPanilItem` - An item added to the application status panel.
* `resourcePanel` - An item added as a resource panel.

Security:

* An extension will be able to make HTTP requests.

See [example](https://github.com/argoproj-labs/argocd-example-extension).

### API Extension

An API extension is:

* A service listening on a port that can be forwarded requests from the server.
* A Kubernetes secret in the `argocd` namespace so each Argo CD server can discover the extension.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: example.extension
  labels:
    argocd.argoproj.io/extension: example
stringData:
  url: http://localhost:3983
  header.Authorization: Bearer ******
  ```

API extensions will have HTTP requests forwarded from `/api/extensions/{name}/{subPath}` to the specified URL.

Security:

* Your service should listen on HTTPS with a recent TLS version, rather than HTTP.
* Your service should mandate an `Authentication` header.

The server does not forward enough information to enforce role-based access controls. The service will not be able to
know which user initiated the request.

See [example](https://github.com/argoproj-labs/argocd-example-extension).