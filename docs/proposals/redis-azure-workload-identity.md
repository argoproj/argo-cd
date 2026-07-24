---
title: Pluggable Redis credentials provider (with Microsoft Entra ID as first implementation)
authors:
  - "@tarapratap"
sponsors:
  - TBD
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2026-06-02
last-updated: 2026-06-04
---

# Pluggable Redis credentials provider (with Microsoft Entra ID as first implementation)

## Summary

Introduce a small `RedisCredentialsProviderFactory` extension point in
`util/cache/` that lets Argo CD acquire short-lived, automatically rotated
Redis credentials from a registered backend (typically a cloud provider's
workload identity), instead of a static password.

A single new flag — `--redis-credentials-provider` — selects which backend is
active. The first registered backend in this proposal is **Microsoft Entra ID
workload identity** for Azure Cache for Redis (`--redis-credentials-provider=azure`).
The same registry trivially accommodates GCP Memorystore IAM and AWS
ElastiCache IAM as future contributions, but those are explicitly out of
scope for this PR.

When a provider is selected, the Redis client uses
`redis.Options.CredentialsProviderContext` to resolve credentials on every
(re)connect, and any value coming from `--redis-password` / `REDIS_PASSWORD`
is discarded. For the `azure` backend the AUTH username is the principal's
object ID (the `oid` JWT claim), and two companion flags
(`--redis-azure-client-id`, `--redis-azure-scope`) handle multi-identity and
sovereign-cloud deployments.

The change is fully opt-in; the flag defaults to empty (= unchanged static
password flow).

## Motivation

Azure Cache for Redis has supported [Microsoft Entra ID
authentication](https://learn.microsoft.com/azure/azure-cache-for-redis/cache-azure-active-directory-for-authentication)
as the recommended auth mode since 2024. Operators on Azure Kubernetes Service
(AKS) typically run every other workload (Cosmos DB, Service Bus, Storage,
Key Vault, …) under [workload identity](https://azure.github.io/azure-workload-identity/docs/),
so secrets never enter cluster state. Argo CD is currently the odd one out: it
requires a long-lived Redis access key plumbed through a Kubernetes `Secret`,
which forces operators to either:

1. Persist the key to source control / Terraform state, or
2. Stand up a side-channel (Key Vault + CSI driver / External Secrets Operator)
   purely to keep that one key out of state.

Both options add operational complexity without delivering security on par with
the workload-identity flow used by the rest of the platform. Several upstream
issues track this gap, e.g.
[argo-cd#19133](https://github.com/argoproj/argo-cd/issues/19133) and
[argo-cd#19136](https://github.com/argoproj/argo-cd/issues/19136). Internally,
the Minecraft Services franchise team — whose adoption motivated this proposal
— hit it during their migration to a shared-Redis topology and worked around
it with a static key, but the wider community has been requesting a first-class
solution.

### Goals

* Let Argo CD authenticate to Azure Cache for Redis without any static
  password, using the same workload identity mechanism that every other Azure
  client SDK in Argo CD already supports (Argo CD already imports
  `github.com/Azure/azure-sdk-for-go/sdk/azidentity`).
* Be fully opt-in; existing deployments are unaffected.
* Reuse the existing `--redis-*` flag surface so the change is uniform across
  `argocd-server`, `argocd-application-controller`, `argocd-repo-server`,
  `argocd-applicationset-controller` and `argocd-notifications-controller`
  (they all share `util/cache.AddCacheFlagsToCmd`).
* Refresh tokens automatically. Entra ID tokens expire roughly every hour; a
  per-`(re)connect` callback fits go-redis' connection lifecycle and avoids
  any custom refresh goroutine.

### Non-Goals

* AWS ElastiCache IAM auth, GCP Memorystore IAM auth, or any other cloud's
  managed-identity-style flow. The same `CredentialsProviderContext` hook
  could host them, but each cloud has its own SDK, scope and identity
  conventions; they belong in follow-up proposals.
* Rotating the `argocd-redis` ConfigMap or any other resource shape change.
* Changing the default authentication path or deprecating
  `--redis-password` / `REDIS_PASSWORD`.
* Workload identity for the bundled in-cluster Redis chart. This proposal
  targets external Redis instances (where AAD auth is meaningful); the
  in-cluster Redis is unauthenticated by default and out of scope.

## Proposal

Introduce a small extension point in `util/cache/`:

```go
type RedisCredentialsProvider = func(ctx context.Context) (username, password string, err error)

type RedisCredentialsProviderFactory interface {
    Name() string                                                  // "azure", "gcp", "aws"
    AddFlags(cmd *cobra.Command, flagPrefix, envPrefix string)     // backend-specific flags
    Build(loadedUsername string) (RedisCredentialsProvider, error) // wired into go-redis
}

func RegisterRedisCredentialsProvider(f RedisCredentialsProviderFactory)
```

Implementations register themselves via `init()`; `AddCacheFlagsToCmd` walks
the registry and exposes both the global selector flag and each backend's
flags. At `Build` time the closure looks up the selected factory by name.

### Flags

| Flag | Env var | Default | Description |
|------|---------|---------|-------------|
| `--redis-credentials-provider` | `REDIS_CREDENTIALS_PROVIDER` | `""` | Use a registered credentials provider for Redis authentication. Currently supported: `azure`. When set, `--redis-password` / `REDIS_PASSWORD` are ignored. |
| `--redis-azure-client-id` | `REDIS_AZURE_CLIENT_ID` | `""` | Optional Entra ID application/client ID override. Defaults to the `AZURE_CLIENT_ID` env var injected by the workload-identity admission webhook. Only honoured when `--redis-credentials-provider=azure`. |
| `--redis-azure-scope` | `REDIS_AZURE_SCOPE` | `https://redis.azure.com/.default` | Override only when targeting non-public Azure clouds (e.g. Azure Government). Only honoured when `--redis-credentials-provider=azure`. |

When `--redis-credentials-provider=azure`, the Azure factory constructs an
`azidentity.NewWorkloadIdentityCredential` (or `NewDefaultAzureCredential`
when no client-id override is provided) and wraps it in a closure that
implements the [`go-redis`](https://github.com/redis/go-redis)
`CredentialsProviderContext` signature. That hook is invoked on every new
connection and on every reconnect, so when the AAD token rotates (every ~60
minutes by default) Argo CD picks up the new token transparently on the next
reconnect — no daemon, no restart, no shared-cache eviction.

The username surfaced to Redis is the principal's **object ID**, decoded from
the `oid` claim of the issued JWT (falling back to `sub` for principals that
do not carry an `oid`). Azure Cache for Redis expects the username in this
form when AAD auth is enabled — neither the application/client ID nor a
human-readable name will work. This decoding is intentionally signature-less:
the caller already trusts the token issuer (azidentity just produced it) and
only needs the payload to recover the AUTH username.

### Use cases

#### Use case 1: External Azure Cache for Redis with workload identity (primary)

A platform team is migrating Argo CD off the bundled in-cluster Redis to a
shared, regionally-replicated Azure Cache for Redis Enterprise instance. They
want every client (including Argo CD) to authenticate the same way as the
rest of their platform: workload-identity-federated AAD tokens, with
`Data Owner` granted to the Argo CD KSA via an `azurerm_role_assignment`.

Setting `--redis-credentials-provider=azure` (or
`REDIS_CREDENTIALS_PROVIDER=azure`) on every Argo CD pod is sufficient. The
existing `azurerm_redis_cache.id` becomes the only thing referenced from
Terraform; no `azurerm_key_vault_secret` and no `data.azurerm_redis_cache`
data source pulling the access key.

#### Use case 2: Azure Government / sovereign cloud

The same operator is running in `usgovvirginia`. They flip
`--redis-azure-scope=https://redis.azure.us/.default` and the default
azidentity Authority probe transparently routes through Azure Government's
identity endpoint.

#### Future use case: GCP Memorystore / AWS ElastiCache

Both [GCP Memorystore for Redis](https://cloud.google.com/memorystore/docs/cluster/auth-using-iam)
and [AWS ElastiCache](https://docs.aws.amazon.com/AmazonElastiCache/latest/dg/auth-iam.html)
support IAM-based authentication that fits the same shape: acquire a
short-lived credential from the cloud SDK, surface it via
`CredentialsProviderContext`. With the registry in place, these become
single-file additions (`util/cache/gcp.go`, `util/cache/aws.go`) that don't
touch `cache.go`. They are intentionally out of scope for this PR — better
to validate the abstraction with one concrete implementation first and let
each cloud's contributors own their own backend.

### Implementation Details

The whole patch lives under `util/cache/`:

* New file **`util/cache/credentials.go`** (~110 lines) containing:
  * `RedisCredentialsProvider` (exported type alias matching go-redis's
    `CredentialsProviderContext` signature).
  * `RedisCredentialsProviderFactory` interface.
  * `RegisterRedisCredentialsProvider` and a private registry guarded by a
    `sync.RWMutex`.
* New file **`util/cache/credentials_test.go`** (~140 lines) covering registry
  registration, duplicate-panic, unknown-name lookup, and `AddFlags`
  delegation.
* New file **`util/cache/azure.go`** (~140 lines) containing:
  * `AzureRedisCredentialsProviderName` and `DefaultAzureCacheForRedisScope`
    (exported constants).
  * `azureCredentialsProviderFactory` struct registered via `init()`, with
    `Name()`, `AddFlags()`, and `Build()` implementing the factory.
  * `newAzureCredential(clientID string) (azcore.TokenCredential, error)` —
    a package-level `var` so tests can substitute a stub.
  * `azureCredentialsProvider(cred, staticUsername, scope) func(ctx) (string, string, error)`.
  * `extractPrincipalID(token) (string, error)` — JWT payload decode that
    prefers `oid` and falls back to `sub`.
* New file **`util/cache/azure_test.go`** (~165 lines) with 13 cases covering
  oid/sub precedence, malformed-token handling, static-username precedence,
  custom-scope plumbing, error propagation, and the per-call refresh
  invariant.
* Modified **`util/cache/cache.go`**:
  * Both `buildRedisClient` and `buildFailoverRedisClient` accept an optional
    `redisCredentialsProvider` parameter and set
    `opts.CredentialsProviderContext` when non-nil. The recursive hook used
    for command-level retries threads the same provider through.
  * `AddCacheFlagsToCmd` registers the new selector flag, calls
    `addRedisCredentialsProviderFlags` to register every backend's flags, and
    inside the existing closure looks the chosen factory up by name and
    invokes its `Build()`.

Argo CD already imports `github.com/Azure/azure-sdk-for-go/sdk/azidentity`
v1.13.x and `github.com/redis/go-redis/v9` v9.18+, so no new module is added.

The flag surface is registered in exactly one place
(`util/cache.AddCacheFlagsToCmd`) and inherited by every binary that uses
the shared cache client; no per-component wiring is required.

### Detailed examples

```yaml
# argocd-cmd-params-cm
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cmd-params-cm
data:
  redis.server: my-cache.redis.cache.windows.net:6380
  redis.use.tls: "true"
  redis.credentials.provider: "azure"
  # redis.azure.client.id: "" — defaults to AZURE_CLIENT_ID
  # redis.azure.scope: "" — defaults to https://redis.azure.com/.default
```

The companion `azurerm_role_assignment` and federated identity credential
shape live outside Argo CD; the Argo CD-side change is just the flags.

### Security Considerations

* **Token lifetime is ~1h.** Even if a token is captured (TLS-protected
  network, in-pod memory access by another container in the same pod), it
  expires fast. Compare to the existing static-key flow where the key is
  long-lived and lives in a Kubernetes `Secret`.
* **No new secret material on disk.** Workload identity tokens live only in
  the projected service-account-token volume that the Kubernetes API server
  refreshes; Argo CD never persists them.
* **JWT decode is signature-less by design.** The decode is purely to read
  the `oid` claim that we just got from azidentity in a TLS-protected call.
  We are not validating identity claims from an untrusted source.
* **Static `--redis-password` is intentionally discarded** when the flag is
  set, so an operator cannot accidentally leave a stale password configured.
  This is documented in the flag help text.
* **Dependency surface.** `azidentity` is already vendored. No new
  third-party crypto code is introduced.

### Risks and Mitigations

| Risk | Mitigation |
|------|-----------|
| Operators select the provider but forget to grant the workload identity `Data Owner` on the cache | First connection fails fast with an Azure-side `WRONGPASS` and a clear log line containing the principal's `oid`; runbook in the operator manual. |
| `oid` claim is missing for unusual principals (e.g. some service principals) | Fallback to `sub`; `errNoUsername` documents the `--redis-username` escape hatch. |
| `azidentity` cannot acquire a token (no AZURE_FEDERATED_TOKEN_FILE) | The error is surfaced at first connect; fallback is to clear `--redis-credentials-provider`. |
| HA Redis with the bundled `redis-ha` haproxy chart | This proposal targets external Redis instances; the in-cluster `redis-ha` haproxy is untouched. |
| Registry abstraction over-engineers a single-cloud need | The interface is deliberately tiny (3 methods). It costs ~110 lines of code to gain a clean extension story for GCP/AWS contributors. |

### Upgrade / Downgrade Strategy

* **Upgrade.** Existing deployments are unaffected: the new flag defaults to
  empty. Operators opt in by setting one ConfigMap key.
* **Downgrade.** Clearing `--redis-credentials-provider` (or rolling back the
  Argo CD image) immediately reverts to static-password auth. The same Redis
  instance can keep AAD enabled in parallel — Azure Cache for Redis allows
  both auth modes simultaneously while in transition.

## Drawbacks

* Adds Azure-specific code to a generic cache package. We argue this is
  acceptable because (a) `azidentity` is already a dependency, (b) it sits
  behind an opt-in flag, (c) it is isolated to `util/cache/azure.go`, and
  (d) the registry interface in `credentials.go` makes it easy to keep the
  Azure code self-contained and add other clouds the same way.
* The registry abstraction has only one implementation today. We accept this
  as a one-time cost: the alternative (refactor on the second cloud) is
  noisier and tends to leak the first implementation's assumptions into the
  abstraction.

## Alternatives

1. **Sidecar Redis-AAD proxy (e.g. open-source `azure-redis-aad-proxy`).** Adds
   an extra hop, an extra container per Argo CD pod, and operator
   responsibility for the proxy's own lifecycle. Doesn't deliver workload
   identity to Argo CD itself.
2. **Out-of-tree fork.** Keeps maintenance burden on individual operators;
   does not benefit the wider community; has to be re-rebased on every Argo
   CD release.
3. **Keep the static-key flow and document a CSI / External Secrets
   pattern.** Works but is the status quo; doesn't move Argo CD onto the
   workload-identity story the rest of the platform has adopted.
4. **Hard-code Azure in `cache.go` (no registry).** Smaller diff but blocks
   GCP/AWS contributors from adding their own backends without churning the
   exact same code. We adopted the registry to keep the door open.

## Open Questions

1. Should the Azure backend live under `util/cache/azure.go` (current
   proposal) or a subpackage `util/cache/providers/azure/`? Subpackage gives
   stricter isolation per cloud; co-location keeps the diff smaller and
   matches the size of the implementation. Happy to move on reviewer
   preference.
2. Should we also accept the principal's object ID via a new
   `--redis-azure-username` flag for environments where the JWT decode is
   undesirable? `--redis-username` already serves this; calling it out
   explicitly may be clearer.
3. Naming: `--redis-credentials-provider=azure` vs
   `--redis-credentials-provider=entra` vs
   `--redis-credentials-provider=aad`. We picked `azure` because that
   matches `azidentity` and `azurerm`; happy to bikeshed.
