# Workload Identity for Repositories

Workload identity lets Argo CD authenticate to Git repositories and OCI/Helm
registries without storing long-lived credentials. Instead of a password or
token in the repository secret, Argo CD requests a short-lived Kubernetes
service account token via the TokenRequest API, exchanges it with a cloud
identity provider (AWS, GCP, Azure, or any OIDC-federated endpoint), and uses
the resulting token to authenticate to the repository.

Identity is scoped per Argo CD project: a repository in project `team-a` is
resolved using the `argocd-project-team-a` service account in the Argo CD
namespace, so each team's repositories can be bound to distinct cloud IAM
identities. A repository with no project uses the service account the Argo CD
component itself runs as.

## How it works

1. A short-lived token for the project-scoped service account
   (`argocd-project-<project>`) is minted via the Kubernetes TokenRequest API.
2. The configured identity provider exchanges that token for a cloud identity
   token (AWS STS credentials, a GCP or Azure access token, or the Kubernetes
   token itself for direct OIDC federation).
3. A repository authenticator turns the identity token into registry
   credentials (an ECR authorization token, an ACR refresh token, the token
   itself for registries that accept it directly, or the response of a
   templated HTTP token exchange).
4. The resulting username/password are injected into the repository and used
   by the repo-server as usual.

Resolved credentials are cached in memory; see [Token caching](#token-caching).

## Repository configuration

Workload identity is enabled by setting `workloadIdentityProvider` in the
repository secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: private-ecr-repo
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  url: 123456789012.dkr.ecr.us-west-2.amazonaws.com/myrepo
  name: my-ecr
  type: helm
  enableOCI: "true"
  project: team-a
  workloadIdentityProvider: aws
```

| Key | Description |
|-----|-------------|
| `workloadIdentityProvider` | Identity provider: `aws`, `gcp`, `azure`, or `k8s`. Required. |
| `workloadIdentityAudience` | Custom audience for the Kubernetes token. Defaults per provider (e.g. `sts.amazonaws.com` for AWS, `api://AzureADTokenExchange` for Azure). |
| `workloadIdentityTokenURL` | Custom token endpoint, e.g. for GovCloud or sovereign clouds. For Azure, `{tenantID}` is substituted. |
| `workloadIdentityUsername` | Username to pair with the resolved token (e.g. a Quay robot account). |
| `workloadIdentityAuthHost` | Override the host used for the token exchange when it differs from the registry host. |
| `workloadIdentityMethod` | HTTP method for the templated exchange (`GET` or `POST`, default `GET`). |
| `workloadIdentityPathTemplate` | URL path template for the exchange, e.g. `/sts/exchange?scope={{ .repo }}`. |
| `workloadIdentityBodyTemplate` | Request body template for `POST` exchanges (form-encoded or JSON, auto-detected). |
| `workloadIdentityAuthType` | How the identity token is sent: `bearer` (default), `basic`, or `none` (template only). |
| `workloadIdentityParams.<name>` | Custom template parameters, available as `{{ .<name> }}`. |
| `workloadIdentityResponseTokenField` | JSON field holding the token in the exchange response. Defaults to trying `access_token`, `token`, then `refresh_token`. |
| `workloadIdentityResponseUsernameField` | JSON field holding the username in the exchange response (e.g. JFrog returns `username`). Takes precedence over `workloadIdentityUsername`. |

The template fields drive the generic HTTP authenticator and are only needed
for endpoints without built-in support; the cloud providers below work with
`workloadIdentityProvider` alone.

## Project service accounts

For each project that uses workload identity, create a service account named
`argocd-project-<project>` in the Argo CD namespace and annotate it for your
cloud provider:

| Provider | Annotation | Value |
|----------|-----------|-------|
| AWS (IRSA) | `eks.amazonaws.com/role-arn` | IAM role ARN to assume |
| GCP | `iam.gke.io/gcp-service-account` | GCP service account email to impersonate |
| GCP (federation) | `iam.gke.io/workload-identity-provider` | Workload identity provider audience, when not on GKE |
| Azure | `azure.workload.identity/client-id` | Entra ID application (client) ID |
| Azure | `azure.workload.identity/tenant-id` | Entra ID tenant ID |

> [!NOTE]
> AWS EKS Pod Identity needs no per-project service account at all: the pod's
> own IAM identity is used, and the repository's project is injected as an
> `argocd-project` session tag on a chained AssumeRole call, which IAM
> policies can reference via `aws:PrincipalTag/argocd-project`.

Repositories without a project fall back to the service account the resolving
component runs as (for example `argocd-server` or
`argocd-application-controller`), determined from the pod's mounted service
account token.

## Providers and authenticators

Each provider has a default way of turning its identity token into repository
credentials:

| Provider | Identity exchange | Repository credentials |
|----------|------------------|------------------------|
| `aws` | EKS Pod Identity, or STS `AssumeRoleWithWebIdentity` (IRSA) | ECR authorization token |
| `gcp` | GKE metadata server, or STS federation, then service account impersonation | Access token used directly (`oauth2accesstoken`) |
| `azure` | Entra ID client-credentials flow with the Kubernetes token as client assertion | ACR refresh token via `/oauth2/exchange`; for `type: git` repositories (Azure DevOps) the access token is used directly |
| `k8s` | None — the Kubernetes service account token is used as-is | Templated HTTP exchange against the registry (or `workloadIdentityAuthHost`) |

The `k8s` provider covers any service that federates directly with the
cluster's OIDC issuer. For example, exchanging the Kubernetes token for a
GitHub token via octo-sts:

```yaml
stringData:
  url: ghcr.io/myorg/myrepo
  type: helm
  enableOCI: "true"
  workloadIdentityProvider: k8s
  workloadIdentityAuthHost: octo-sts.dev
  workloadIdentityPathTemplate: "/sts/exchange?scope={{ .repo }}&identity={{ .policy }}"
  workloadIdentityParams.policy: argocd
  workloadIdentityUsername: x-access-token
```

## Token caching

Resolved repository credentials are cached in memory per component, so
repeated reconciliations do not repeat the token exchanges. The cache entry
lifetime is derived from the token's reported expiry minus a five-minute
safety margin; when the endpoint reports no expiry, a conservative default is
used. Every entry is additionally capped by a maximum TTL, which bounds how
long a revoked token or a rotated service account annotation can keep being
served from cache.

Both values are configurable on the `argocd-server` and
`argocd-application-controller` commands:

| Flag | Environment variable | Default | Description |
|------|---------------------|---------|-------------|
| `--workload-identity-token-cache-ttl` | `ARGOCD_WORKLOAD_IDENTITY_TOKEN_CACHE_TTL` | `40m` | TTL for credentials whose expiry is not reported by the token endpoint. `0` disables caching of such credentials. |
| `--workload-identity-token-cache-max-ttl` | `ARGOCD_WORKLOAD_IDENTITY_TOKEN_CACHE_MAX_TTL` | `1h` | Upper bound on how long any credential is served from cache, regardless of token lifetime. |

> [!WARNING]
> Raising the max TTL widens the window during which a revoked cloud identity
> (for example, a rotated `eks.amazonaws.com/role-arn` annotation) continues
> to be honored from cache. Credentials are keyed by the repository's
> workload identity configuration, so changing any of those fields takes
> effect immediately; changes made only on the cloud side take effect when
> the cache entry expires.
