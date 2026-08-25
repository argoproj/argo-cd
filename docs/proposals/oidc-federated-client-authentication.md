---
title: Federated Client Authentication for OIDC
authors:
  - "@ppapapetrou76"
sponsors:
  - TBD
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2026-08-24
---

# Federated Client Authentication for OIDC

Let the Argo CD API server authenticate to an OIDC provider with a short-lived JWT issued by
something else, in practice a projected Kubernetes ServiceAccount token, instead of a static
`clientSecret` in `argocd-secret`. Tracking issue:
[#28935](https://github.com/argoproj/argo-cd/issues/28935).

## Open Questions

* Keycloak-specific or provider-neutral config? The issue proposes a
  `keycloak.clientAuthentication` block but I'd rather have a provider-neutral
  `clientAuthentication` implementation: it's plain RFC 7523, and we already have it in Entra ID
  under `azure.useWorkloadIdentity`. A Keycloak-shaped key leaves us with three names for one
  mechanism. This is the only real design decision in here. Everything else follows from it, so I'd
  like to get this finalized before diving to the code.
* Where does the token path come from? `tokenFile` in `argocd-cm` is the obvious answer, and it's
  what the issue proposes. It also means a ConfigMap edit decides which file gets read and where the
  contents go, which is easier to reach than I'd like (see Security Considerations). Supporting
  `tokenFileEnv` as well, and recommending it, puts the path back in the Pod spec for the price of
  one extra key. So: both, or only the env var that Entra ID already uses? I lean toward both, but
  not strongly.
* Should the refresh-grant fix land separately? The assertion has to go on the `refresh_token` grant
  as well as the code exchange because currently there's an existing bug for Azure Workload
  Identity users and has nothing to do with Keycloak. This fix can be done in a separate PR and also
  backported. That would leave this proposal as a configuration and nothing else.

## Summary

Keycloak 26.6 made [federated client authentication][kc-blog] supported: a client can authenticate
at the token endpoint with a JWT issued by a third party the realm trusts, instead of its own client
secret. On Kubernetes that third party is the cluster, and the JWT is a projected `ServiceAccount`
token.

This proposal adds a `clientAuthentication` block to `oidc.config` picking between today's
secret-based auth and a federated JWT read from disk. Choose the federated method, and every call to
the token endpoint carries
`client_assertion_type=urn:ietf:params:oauth:client-assertion-type:jwt-bearer` plus
`client_assertion=<token>` in place of client credentials. `azure.useWorkloadIdentity` becomes an
alias for the same code path.

The mechanism isn't Keycloak-specific. It's [RFC 7521][rfc7521] / [RFC 7523][rfc7523] client
authentication, the same flow the code already supports for Entra ID. Keycloak is just what prompted
it: a common Argo CD IdP, and the feature is supported now instead of preview.

## Motivation

The SSO client secret has some existing issues, such as never expiring and no rotation path.
Anyone holding `get secrets` in the Argo CD namespace can read it, and it stays valid until a human
changes it. If we compare this to a projected `ServiceAccount` token, we can tell that it is only
readable from inside the Pod. Then there is another case where things run secret-less everywhere
else, workload identity for cloud APIs, cert-manager for TLS, and the current implementation for SSO
needs to be handled differently.

We already solved this once with `azure.useWorkloadIdentity` which reads a projected token from
`AZURE_FEDERATED_TOKEN_FILE` and sends it as a client assertion (`util/oidc/oidc.go`).

### Goals

1. Authenticate to the OIDC provider's token endpoint with an externally issued JWT read from a
   file, with no client secret configured.
2. Send the assertion on every grant Argo CD uses against the token endpoint, including the
   `refresh_token` grant that backs `refreshTokenThreshold`. This currently does not happen for
   Azure Workload Identity either, so this proposal treats fixing it as in scope.
3. Re-read the token from disk as the kubelet rotates it, without restarting `argocd-server`.
4. Express the feature so that it works for any provider accepting third-party client assertions,
   and fold the existing Entra ID support into it without breaking existing configuration.
5. Fail with a message that says what is wrong. Every misconfiguration in this area shows up as a
   bare `invalid_client` from the provider today.

### Non-Goals

1. Dex. Deployments that reach Keycloak through the bundled Dex keep a client secret in the Dex
   connector config. Dex has no federated assertion support, and this proposal does not add
   one. Only the direct `oidc.config` path is covered.
2. The CLI. `argocd login --sso` already authenticates as a public client with PKCE using
   `cliClientID`, and the client secret is never sent to it. Nothing changes here.
3. SPIFFE. Keycloak's SPIFFE variant is still a preview feature while the spec is a draft. A JWT
   SVID would drop into the file-based design unchanged, but nothing here is built or tested for
   it.
4. Self-signed client assertions (`private_key_jwt`). Still a long-lived private key, so it doesn't
   solve the problem.

## Proposal

### Use cases

#### Use case 1:
As an Argo CD operator, I want to run SSO against Keycloak without storing a client secret, so that
there is no credential to rotate and nothing for a namespace reader to steal.

#### Use case 2:
As a platform team with a policy against long-lived credentials, I want the Argo CD to IdP trust to
be expressed as "this ServiceAccount in this cluster", so that it is auditable and can be revoked by
changing the identity provider configuration in Keycloak instead of rotating a secret.

### Implementation Details/Notes/Constraints

#### How the flow works

The JWT goes over verbatim in `client_assertion`, on an otherwise ordinary form-encoded POST to the
token endpoint.

Keycloak checks the signature first, against the JWKS of the identity provider registered in the
realm. Then the claims. Two of them are ours to get right:

* `sub` must match the external subject configured on the Keycloak client. For a projected token
  that's `system:serviceaccount:argocd:argocd-server`. The external subject is a single value, so
  two Argo CD instances in two clusters cannot share one Keycloak client. Each needs its own client,
  and probably its own identity provider.
* `aud` must be a single value, equal to the realm issuer URL.

Keycloak also wants `exp` in the future, `iat` no more than 300 seconds in the past, and a `jti`
unless the identity provider permits reuse. The full list of what Keycloak validates is in
[keycloak/keycloak#42634][kc-design].

Kubernetes `ServiceAccount` tokens carry no `jti`, so the realm's Kubernetes identity provider has
to permit reuse. Replay resistance therefore comes from TLS and from a short token lifetime, which
is why `expirationSeconds: 600` is the right setting here. It is also the floor: projected tokens
[default to an hour and must be at least ten minutes][k8s-projected].

Keycloak also has to be able to fetch the cluster's `/.well-known/openid-configuration` and JWKS.
Some managed Kubernetes offerings expose a public issuer, either by default or behind a flag. On a
self-managed cluster the operator either exposes the issuer or pastes the JWKS into the identity
provider configuration by hand. There is no way for Argo CD to work around it.

#### Configuration

```yaml
oidc.config: |
  name: Keycloak
  issuer: https://keycloak.example.com/realms/argocd
  clientID: argocd
  clientAuthentication:
    method: federated_jwt          # client_secret (default) | federated_jwt
    federatedJWT:
      tokenFileEnv: ARGOCD_OIDC_FEDERATED_TOKEN_FILE
      # or, if the path is fixed and known:
      # tokenFile: /var/run/secrets/tokens/keycloak-token
      # audience: defaults to the configured issuer, which is what Keycloak wants.
      # Set it when the provider expects something else.
  requestedScopes: ["openid", "profile", "email", "groups"]
```

`method` defaults to `client_secret`, so every existing configuration keeps working untouched.
`azure.useWorkloadIdentity: true` resolves internally to `method: federated_jwt` with
`tokenFileEnv: AZURE_FEDERATED_TOKEN_FILE` and `audience: api://AzureADTokenExchange`, which keeps
the Entra ID configuration valid and puts both providers on one code path.

`audience` is what the token's `aud` claim is checked against before the assertion is sent. It
defaults to the configured `issuer`, which is right for Keycloak and wrong for Entra ID, hence the
alias supplying its own. Nobody configuring Keycloak needs to set it.

`oidc.config` is an opaque string in `argocd-cm` parsed by `util/settings`, so this needs no API
type change and no `make codegen`. It must not reach `settingspkg.OIDCConfig`, the struct the API
server hands to clients: `SettingsService/Get` is deliberately unauthenticated so the login page can
call it, so anything added there publishes the token path and the environment variable name to
anyone who asks.

`ValidateOIDCConfig` should reject `method: federated_jwt` alongside a non-empty `clientSecret`,
neither or both of `tokenFile` / `tokenFileEnv`, and an unrecognized `method`. Note that "reject"
overstates what validation does today: `updateSettingsFromConfigMap` logs the failure and loads the
config anyway. So each case also needs defined runtime behavior. `federated_jwt` plus a
`clientSecret` uses the assertion and logs that the secret is ignored; an unrecognized `method`
refuses to authenticate instead of quietly falling back.

#### Where the assertion is injected

Today the Azure assertion is appended as an `oauth2.AuthCodeOption` at the one call site that
performs the code exchange, in `HandleCallback`. That is why refresh does not work:
`GetTokenSourceFromCache` builds an `oauth2.Config` from `getOauth2ConfigForRedirectURI` and calls
`config.TokenSource(...)`, which knows nothing about assertions and authenticates with an empty
client secret. Against Keycloak that is an `invalid_client`.

That is worse than a session quietly failing to extend. `CheckAndRefreshToken` is called from the
gRPC auth interceptor, twice, and both call sites swallow the error into a log line. The first one
fires reactively whenever `VerifyToken` rejects an expired ID token, so once a user's token lapses,
every request they make produces another failing POST to the token endpoint. There is no backoff
anywhere in `util/oidc`. The result is IdP load proportional to request rate, a stream of warnings
in the server log, and a user who just sees their session end. Setting `refreshTokenThreshold`, as
the tracking issue's example does, makes it proactive as well.

We could add the same options at the second call site. Better to wrap the HTTP transport in a
`RoundTripper` that injects the assertion into form-encoded POSTs bound for the token endpoint, and
leaves every other request over that client alone. `ClientApp.client` already reaches every OIDC
call through `gooidc.ClientContext(ctx, a.client)`, so wrapping it once in `NewClientApp` covers the
code exchange, the refresh grant, and whatever gets added later. The Azure `AuthCodeOption` block
then goes away.

Several details around that are easy to get wrong: the token endpoint is not known when the
transport is built, `RoundTripper` may not mutate the request it is handed, and an empty
`ClientSecret` can still produce a Basic auth header that Keycloak rejects. They are implementation
notes on the tracking issue, not design decisions.

#### Reading and caching the token

The kubelet rewrites the projected token periodically, so `argocd-server` must re-read the file
instead of caching the first value it saw. The existing Azure helper caches for a flat ten minutes,
on the reasoning that the shortest token lifetime is an hour. That doesn't survive a ten-minute
token: cache a 600-second token for 600 seconds from the moment you read it and it expires before
the cache does. Parse `exp` from the file and cache until shortly before it.

Two checks should not wait for the first login. At startup, confirm the file exists and parses as a
JWT. Before the token is ever sent, confirm its `aud` is the one the provider expects, since
forgetting `audience:` on the projected volume turns into an opaque `invalid_client`.

Both should log instead of failing. `server.go` wraps `NewClientApp` in `errorsutil.CheckError`, so
an error there kills the process, and `argocd admin dashboard` and `--core` build a `ClientApp` on
the operator's laptop from the cluster's real `argocd-cm`, where the projected token will never
exist. A
fatal check breaks those commands on a perfectly good configuration.

"The one the provider expects" is why `federatedJWT.audience` exists at all. Keycloak wants the
realm issuer URL, so the default is right there. Entra ID does not: an Azure Workload Identity token
carries `aud: api://AzureADTokenExchange` while `issuer` points at `login.microsoftonline.com`.
Compare against `issuer` unconditionally and every existing `azure.useWorkloadIdentity` user stops
being able to log in the moment they upgrade.

#### What does not change

The CLI is unaffected: it already authenticates as a public client with PKCE using `cliClientID`,
and the API server never sends it a client secret. As today, a confidential `clientID` means
operators need a separate public client behind `cliClientID`. Logout, Dex-backed deployments and
`argocd-server`'s RBAC are all untouched.

#### What the documentation has to cover

A new section in `docs/operator-manual/user-management/keycloak.md`, next to the existing client
authentication and PKCE ones, with the prerequisites and realm setup, a `> [!WARNING]` block on the
trust and replay trade-offs, and a troubleshooting table for `invalid_client`. `microsoft.md` gets a
note that the Entra ID setup can be written in the provider-neutral form too.

### Detailed examples

**Keycloak realm setup** (26.6 or later), summarized. These steps are reconstructed from Keycloak's
announcement and design issue, not transcribed from the admin guide, so the exact console labels
should be confirmed against a 26.6 instance before any of this reaches user docs:

1. Create an identity provider of type *Kubernetes* in the realm, pointing at the cluster's service
   account issuer URL. Keycloak fetches the JWKS from the issuer's discovery document; for clusters
   whose issuer is not reachable from Keycloak, supply the JWKS manually.
2. On the `argocd` client, set client authentication to federated, select the identity provider
   above, and set the external subject to `system:serviceaccount:argocd:argocd-server`.
3. Leave replay protection off for this identity provider, since Kubernetes tokens have no `jti`.

**Projecting the token into `argocd-server`** (Helm values):

```yaml
server:
  volumes:
    - name: keycloak-token
      projected:
        sources:
          - serviceAccountToken:
              audience: https://keycloak.example.com/realms/argocd
              expirationSeconds: 600
              path: keycloak-token

  volumeMounts:
    - name: keycloak-token
      mountPath: /var/run/secrets/tokens
      readOnly: true

  env:
    - name: ARGOCD_OIDC_FEDERATED_TOKEN_FILE
      value: /var/run/secrets/tokens/keycloak-token
```

The `audience` must be the realm issuer URL and must be the only audience requested, since Keycloak
requires `aud` to be a single value. `expirationSeconds: 600` is the lowest Kubernetes allows.

The `oidc.config` block is the one under Configuration above, plus a `cliClientID`. No
`argocd-secret` entry is needed, and `oidc.keycloak.clientSecret` can be deleted once the migration
is verified.

### Security Considerations

The token path is the one part of this design with a real attack against it.

`argocd-cm` is a ConfigMap, and editing one takes less privilege than editing the `argocd-server`
Deployment or reading `argocd-secret`. An attacker with that access can set
`tokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token` and point `issuer` at a host they
control. Argo CD then posts its real Kubernetes API credential to them. That is a jump from
ConfigMap edit to Argo CD's cluster identity, and it is why this proposal recommends `tokenFileEnv`:
the path comes from the Pod spec, the way Entra ID already does it. It needs an allowlisted prefix
(`/var/run/secrets/`), no traversal, no symlink escapes, and a refusal to send any token whose
audience is not the one the provider expects.

`tokenFileEnv` is better, not safe. The path lives in the Pod spec, but which environment variable
gets dereferenced still comes from `argocd-cm`, so an attacker picks from whatever the pod has set.
Same validation, both forms.

The rest:

* The credential becomes "who can run a Pod as this ServiceAccount". Anyone who can create a Pod in
  the Argo CD namespace with `serviceAccountName: argocd-server` can get an assertion. Roughly the
  same people can read `argocd-secret` today, so it's not a regression. The credential moved. It
  didn't disappear.
* That comparison assumes the Keycloak client only does authorization-code flow. Turn on service
  account roles and the assertion also buys a `client_credentials` grant; turn on token exchange and
  it buys more. That belongs in the docs as a requirement on the Keycloak client.
* Keycloak now trusts the cluster's signing keys, which means a compromised control plane can mint
  an assertion for any client whose external subject is a ServiceAccount in that cluster. Keycloak's
  own docs flag this.
* No replay protection. Kubernetes tokens have no `jti`, so an intercepted assertion works until it
  expires. TLS and the ten-minute lifetime are all that limit the window, and only one of those is
  enforced: `ValidateExternalURL` accepts `http` as readily as `https`, so `issuer:
  http://keycloak.internal` ships the assertion in cleartext with nothing objecting. Either require
  `https` for `federated_jwt`, or stop counting TLS as a control.
* Exposing the cluster issuer publishes public keys and nothing else, but plenty of security teams
  will still want to sign off on it.

### Risks and Mitigations

Everything fails the same way. Missing file, wrong audience, wrong external subject, expired token,
Keycloak older than [26.6][kc-release-notes], clock skew outside the 300 second `iat` window: it
all comes back as a
bare `invalid_client`. The local checks catch the two likeliest causes before the request leaves the
process. The rest needs a troubleshooting table in the docs, and a counter would help, since there
is nothing today that says which authentication method is live or how often assertions are being
rejected.

The worst failure mode is a crash loop. Any `oidc.config` edit restarts the API server by design,
and `NewClientApp` errors are fatal, so a startup check that returns an error on a missing token
file turns one bad `argocd-cm` edit into a restart loop with the operator locked out of the UI and
CLI they would use to revert it. Hence logging instead of failing, and a test for it: break the
mount, confirm the server still serves.

Folding `azure.useWorkloadIdentity` into the shared path means touching code that works today. The
flag stays an alias and nothing changes for existing users except that refresh starts working, but a
test should assert the alias still produces the request it produces now.

One thing needs checking against a live 26.6 instance before implementation: `oauth2` keeps sending
`client_id` from the config while Keycloak looks the client up from the assertion's `sub`, and
whether it tolerates both is not documented anywhere I could find.

### Upgrade / Downgrade Strategy

There is nothing to upgrade. `clientAuthentication` is absent from every existing configuration and
defaults to `client_secret`, which is exactly today's behavior, and there are no API type or
manifest changes.

Adopting the feature is a two-step migration an operator can spread out:

1. Configure the identity provider and the client in Keycloak, project the token into
   `argocd-server`, and switch `oidc.config` to `method: federated_jwt`. Both halves cost a restart:
   the projected volume needs a rollout, and any `oidc.config` edit makes `watchSettings` restart
   the API server on purpose.
2. Once logins work, delete the client secret from `argocd-secret` and from the Keycloak client.

Rolling back is the reverse and equally safe: restore the client secret, remove the
`clientAuthentication` block. Nothing persists any state tied to the authentication method. Existing
sessions do keep working, though not because they are left alone: an expired ID token gets
re-exchanged through the refresh grant in the auth interceptor, and once the client secret is back
that exchange authenticates the old way again.

Downgrading with `method: federated_jwt` still set fails logins until the client secret is
restored, like any other unknown `oidc.config` key.

## Drawbacks

* One more authentication mode in code that already has several: client secret, PKCE, Azure workload
  identity, Dex. It replaces the Azure branch instead of adding to it, so the count holds steady,
  but it only works on Kubernetes, and only with providers that take third-party assertions.
* When it breaks, it breaks somewhere else. Debugging a rejected assertion means reading Keycloak
  logs, which most Argo CD operators never have to do.
* Configuration ends up in three places. The projected volume, the audience and the ServiceAccount
  name all have to line up with the Keycloak client, and none of that lives in `argocd-cm`.
* Direct-to-provider deployments only. Anyone reaching Keycloak through Dex gets nothing.

## Alternatives

* Keep the issue's `keycloak.clientAuthentication` shape. It matches how `azure.useWorkloadIdentity`
  reads today, and it's probably easier to find if you got here from the Keycloak docs. The cost is
  a third vendor block the next time some provider ships this, and no shared path with Entra ID. The
  implementation underneath is identical either way; only the parsing differs.
* `private_key_jwt` with a cert-manager-managed key. Keycloak has supported self-signed assertions
  for years and cert-manager could rotate the key. You still hold a long-lived private key, and now
  you own a certificate lifecycle too. That's more to run for a smaller gain.
* Rotate the client secret through Keycloak's admin API. That means handing Argo CD admin
  credentials on the realm, a bigger credential than the one being rotated.
* Do nothing. Client secrets work and the Keycloak feature is new, so it's a fair position. The
  counter is that we already ship this mechanism for one provider, so the marginal cost is small.

[rfc7521]: https://datatracker.ietf.org/doc/html/rfc7521
[rfc7523]: https://datatracker.ietf.org/doc/html/rfc7523
[kc-blog]: https://www.keycloak.org/2026/01/federated-client-authentication
[kc-release-notes]: https://www.keycloak.org/docs/latest/release_notes/index.html#federated-client-authentication-supported
[kc-design]: https://github.com/keycloak/keycloak/issues/42634
[k8s-projected]: https://kubernetes.io/docs/concepts/storage/projected-volumes/
