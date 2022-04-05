# API Extensions

An API extension is:

* A service listening on a port that can be forwarded requests from the server.
* A Kubernetes secret in the `argocd` namespace so each Argo CD server can discover the extension.

Only install extensions from trusted sources.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: example.extension
  labels:
    # this label is the unique name of the extension
    argocd.argoproj.io/extension: example
stringData:
  # the target URL for requests
  url: http://localhost:3983
  # headers to add to each request
  headers: |
    # may from header name to array of values (it is very rare to have >1 value)
    Authorization: 
    - Bearer LetMeIn
  # paths that this extension supports
  paths: |
    # the path, this will become /api/extensions/example/application/my-app
    "/applications/{application}":
      GET: 
        # RBAC resource name: clusters,projects,applications,repositories,certificates,accounts,gpgkeys,logs
        resource: applications
        # RBAC action: get,create,update,delete,sync,override,actino
        action: get
        # RBAC object: this must be in the path with curly-braces, e.g. {application}
        object: application
  ```

API extensions will have HTTP requests forwarded from `/api/extensions/{name}/{subPath}` to the specified URL.

## Security:

* Your service should listen on HTTPS with a recent TLS version, rather than HTTP.
* Your service should mandate an `Authentication` header.

Assuming Argo CD has authorization enabled, your service will receive authenticated requests. 

See [example](https://github.com/argoproj-labs/argocd-example-extension).