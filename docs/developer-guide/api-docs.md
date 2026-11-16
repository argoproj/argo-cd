# API Docs

You can find the Swagger docs by setting the path to `/swagger-ui` in your Argo CD UI. E.g. [http://localhost:8080/swagger-ui](http://localhost:8080/swagger-ui).

## Authorization

You'll need to authorize your API requests using a bearer token. To get a token:

```bash
$ curl -H "Content-Type: application/json" $ARGOCD_SERVER/api/v1/session -d $'{"username":"admin","password":"password"}'
{"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpYXQiOjE1Njc4MTIzODcsImlzcyI6ImFyZ29jZCIsIm5iZiI6MTU2NzgxMjM4Nywic3ViIjoiYWRtaW4ifQ.ejyTgFxLhuY9mOBtKhcnvobg3QZXJ4_RusN_KIdVwao"} 
```

> [!NOTE]
> `/api/v1/session` authenticates local accounts only. Password login requires a local account
> that is enabled, has a password set, and has the `login` capability: the built-in `admin`
> account (enabled unless `admin.enabled` is set to `false` in `argocd-cm`), or an
> `accounts.<name>` entry in `argocd-cm` declaring `login`. Accounts that are disabled, have no
> password, or are configured only with the `apiKey` capability — such as API-only service
> accounts — are rejected. It does not authenticate SSO identities. If Argo CD is configured with
> Dex (LDAP, SAML, GitHub, ...) or an external OIDC provider, posting an SSO user's username and
> password here returns `Invalid username or password`, which is expected: those identities are
> verified by the identity provider through a browser redirect flow, not by Argo CD.
>
> For headless and CI use with SSO, see
> [CI/CD Pipeline Authentication](../operator-manual/user-management/index.md#cicd-pipeline-authentication),
> which covers the full set of options. Which apply depends on how Argo CD is configured:
> a [project role token](../operator-manual/user-management/index.md#project-role-tokens) or a
> [local user API token](../operator-manual/user-management/index.md#local-user-api-token) work in
> any installation; [Dex Token Exchange](../operator-manual/user-management/index.md#dex-token-exchange)
> requires Dex; an
> [external OIDC ID token](../operator-manual/user-management/index.md#external-oidc-id-token) applies
> when Argo CD talks to an OIDC provider directly through `oidc.config`; and
> [`--core` mode](../operator-manual/user-management/index.md#kubernetes-direct-core-mode) bypasses
> Argo CD authentication in favor of Kubernetes RBAC.

Then pass using the HTTP `Authorization` header, prefixing it with `Bearer `:

```bash
$ curl $ARGOCD_SERVER/api/v1/applications -H "Authorization: Bearer $ARGOCD_TOKEN" 
{"metadata":{"selfLink":"/apis/argoproj.io/v1alpha1/namespaces/argocd/applications","resourceVersion":"37755"},"items":...}
```

## Services

### Applications API

#### How to Avoid 403 Errors for Missing Applications

All endpoints of the Applications API accept an optional `project` query string parameter. If the parameter 
is specified, and the specified Application does not exist, the API will return a `404` error.

Additionally, if the `project` query string parameter is specified and the Application exists but is not in 
the given `project`, the API will return a `403` error. This is to prevent leaking information about the 
existence of Applications to users who do not have access to them.
