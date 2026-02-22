---
title: Workload Identity for Repository Authentication
authors:
  - "@blakepettersson" # Replace with actual GitHub handle
sponsors:
  - TBD
reviewers:
  - "@sabre1041"
  - TBD
approvers:
  - TBD

creation-date: 2025-01-21
last-updated: 2025-01-29
---

# Workload Identity for Repository Authentication

Enable Argo CD to authenticate to OCI registries and Git repositories using cloud-native workload identity instead
of static credentials.

## Summary

This proposal introduces workload identity support for Argo CD repository authentication. Instead of storing long-lived
credentials (passwords, tokens, service account keys) in Kubernetes secrets, Argo CD can authenticate to registries using
short-lived tokens obtained through cloud provider workload identity mechanisms (AWS IRSA, GCP Workload Identity,
Azure Workload Identity) or OIDC federation (SPIFFE/SPIRE, direct K8s OIDC).

The implementation adds a `workloadIdentityProvider` field to repository configuration to specify which identity
mechanism to use. Credentials are resolved at runtime by exchanging Kubernetes service account tokens for
registry-specific credentials.

## Motivation

Modern cloud-native environments are moving away from static credentials toward identity-based authentication. This
shift provides significant security benefits. While there is some ad-hoc support for it in some places, the existing
implementations of it have a few issues.

**Current State:**
- Argo CD stores registry credentials (username/password, tokens) in Kubernetes secrets
- These credentials are long-lived and must be manually rotated
- Credential leakage poses significant security risks
- The existing implementation of Workload Identity is in practice scoped on the whole repo-server, meaning that there
is no granularity between projects.
- There is only a single existing implementation for Azure, lacking support for other clouds as well as on-prem
- The repo-server is the entry point of the existing implementation, breaking its "dumbness" (it is only supposed to
generate manifests from credentials given, not wrangle with workload identity, needing to add IAM setup to it etc.)

**Desired State:**
- Zero static credentials stored for registry access
- Automatic credential rotation through short-lived tokens
- Per-project identity isolation using Kubernetes service accounts
- Native integration with cloud provider identity systems

### Goals

1. **Eliminate static credentials**: Enable repository authentication without storing long-lived passwords or tokens in secrets.
2. **Support major cloud providers**:
Implement native support for:
   - AWS IRSA (IAM Roles for Service Accounts) for ECR
   - GCP Workload Identity Federation for Artifact Registry
   - Azure Workload Identity for ACR
3. **Support SPIFFE/SPIRE**: Enable workload identity using SPIFFE JWT-SVIDs with delegated identity for per-project isolation.
4. **Support generic token exchange**: Enable authentication to registries that supports exchanging an
  ID token for repo creds (e.g. Quay, JFrog Artifactory, etc.) via a flexible HTTP template authenticator.
5. **Per-project isolation**: Each ArgoCD project can use a different identity, allowing fine-grained access control
  at the cloud IAM level.
6. **Backward compatibility**: Existing repositories with static credentials continue to work unchanged.
7. **Credential caching (potentially?)**: Token caching with TTL management.

### Non-Goals

1. **Automatic cloud IAM setup**: Users must configure cloud provider IAM roles/policies manually.
2. **Automatic configuration of service accounts**: An Argo CD admin needs to (manually) provision k8s service accounts, 
at least for now. A future enhancement could be to automatically provision a service account whenever an AppProject is 
created.

## Proposal

### Use Cases

#### Use case 1: AWS ECR with IRSA
As an operator running ArgoCD on EKS, I want to authenticate to ECR without storing AWS credentials, using IAM roles 
mapped to Kubernetes service accounts.

#### Use case 2: GCP Artifact Registry with Workload Identity
As an operator running ArgoCD on GKE or any Kubernetes cluster, I want to authenticate to Artifact Registry using GCP 
Workload Identity Federation without service account keys.

#### Use case 3: Azure ACR with Workload Identity
As an operator running ArgoCD on AKS, I want to authenticate to ACR using Azure Workload Identity without storing 
service principal secrets.

#### Use case 4: SPIFFE/SPIRE with Quay
As an operator using SPIRE for workload identity, I want ArgoCD to authenticate to Quay using SPIFFE JWT-SVIDs with 
per-project SPIFFE identities.

#### Use case 5: Multi-tenant isolation
As a platform team, I want different Argo CD projects to use different cloud IAM roles, so project A can only access 
production ECR repositories while project B can only access staging repositories.

### Implementation Details

#### Architecture

The implementation follows a two-phase provider-based architecture:

1. **Identity Provider**: Exchanges K8s service account tokens for cloud/registry credentials
2. **Repository Authenticator**: Uses the identity token to authenticate to the specific repository type

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Repository Configuration                             │
│  workloadIdentityProvider: "aws" | "gcp" | "azure" | "spiffe" | "k8s"       │
│  project: "production"                                                       │
└─────────────────────────────────────────────────────────────────────────────┘
│
▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Workload Identity Resolver                           │
│  1. Lookup service account: argocd-project-{project}                         │
│  2. Read provider annotations from service account                           │
│  3. Select identity provider and repository authenticator                    │
└─────────────────────────────────────────────────────────────────────────────┘
│
┌─────────────────────┴─────────────────────┐
▼                                           ▼
┌─────────────────────────────┐           ┌─────────────────────────────────┐
│      Identity Provider      │           │    Repository Authenticator     │
│  - Request K8s token via    │           │  - ECR (GetAuthorizationToken)  │
│    TokenRequest API         │──────────▶│  - ACR (token exchange)         │
│  - Exchange for cloud creds │           │  - Passthrough (GCP, etc.)      │
│                             │           │  - HTTP Template (Quay, Harbor) │
└─────────────────────────────┘           └─────────────────────────────────┘
│                                              │
│                                              ▼
┌────┴────┬────────┬────────┬────────┐    ┌─────────────────────┐
▼         ▼        ▼        ▼        ▼    │ Credentials         │
┌───────┐ ┌───────┐ ┌───────┐ ┌───────┐ ┌───┐│ {Username, Password}│
│  AWS  │ │  GCP  │ │ Azure │ │SPIFFE │ │K8s││ returned to caller  │
│(IRSA) │ │ (WIF) │ │ (WI)  │ │       │ │   │└─────────────────────┘
└───────┘ └───────┘ └───────┘ └───────┘ └───┘
```

#### Service Account Naming Convention

Each Argo CD project maps to a Kubernetes service account:

```
Project Name                   → Service Account Name
"production"                   → argocd-project-production
"staging"                      → argocd-project-staging
""  (non-scoped credential)    → argocd-global
```

#### Identity Providers

| Provider | Description                                                |
|----------|------------------------------------------------------------|
| `aws` | AWS STS AssumeRoleWithWebIdentity for IRSA                 |
| `gcp` | GCP Workload Identity Federation                           |
| `azure` | Azure Workload Identity                                    |
| `spiffe` | SPIFFE/SPIRE workload identity                             |
| `k8s` | Direct K8s JWT passthrough for registries that accept JWTs |

#### Repository Authenticators

| Authenticator | Description |
|---------------|-------------|
| `ecr` | AWS ECR GetAuthorizationToken |
| `acr` | Azure Container Registry token exchange |
| `passthrough` | Direct token passthrough (for GCP Artifact Registry, etc.) |
| `http` | HTTP template-based authenticator for custom endpoints (Quay, JFrog, etc.) |

Each identity provider has a default authenticator, but this can be overridden if needed.

#### Registry OIDC Federation Support

The following registries support OIDC workload identity federation, enabling authentication with Kubernetes 
ServiceAccount tokens or SPIFFE JWT-SVIDs:

| Registry | OIDC Federation Support | Notes |
|----------|------------------------|-------|
| **Quay** | Robot Account Federation | Maps OIDC claims to robot accounts |
| **JFrog Artifactory** | OIDC token exchange | Supports custom identity providers |
| **AWS ECR** | IRSA via STS | Requires AssumeRoleWithWebIdentity |
| **GCP Artifact Registry** | Workload Identity Federation | Native GCP WIF support |
| **Azure ACR** | Workload Identity | Native Azure AD integration |

For registries with native bearer token OIDC support (like Zot), the K8s JWT can be used directly without an
intermediate token exchange. For registries that require token exchange (like Quay robot federation), the HTTP
template authenticator handles the exchange.

#### Provider-Specific Flows

**AWS (IRSA):**
1. Request K8s token with audience `sts.amazonaws.com`
2. Call STS `AssumeRoleWithWebIdentity` with the K8s JWT
3. Use temporary credentials to call ECR `GetAuthorizationToken`
4. Return ECR credentials (username: `AWS`, password: base64-decoded token)

**GCP (Workload Identity Federation):**
1. Request K8s token with WIF provider audience
2. Exchange K8s JWT for federated token via GCP STS
3. Impersonate target GCP service account
4. Return credentials (username: `oauth2accesstoken`, password: access token)

**Azure (Workload Identity):**
1. Request K8s token with audience `api://AzureADTokenExchange`
2. Exchange K8s JWT for Azure access token via Azure AD
3. Exchange Azure token for ACR refresh token
4. Return ACR credentials

**SPIFFE/SPIRE:**
1. Fetch JWT-SVID for project's SPIFFE ID
2. Use HTTP template authenticator to exchange JWT for registry token
3. Return credentials

**K8s (Direct OIDC):**
1. Request K8s token with configured audience
2. Use HTTP template authenticator to exchange JWT for registry token
3. Return credentials

#### Repository Secret Configuration

Fields added to repository secrets:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-ecr-repo
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  type: helm
  url: oci://123456789012.dkr.ecr.us-west-2.amazonaws.com/charts
  project: production

  # Workload identity provider (enables workload identity when set)
  workloadIdentityProvider: "aws"  # aws, gcp, azure, spiffe, k8s

  # Optional: Override token endpoint URL
  workloadIdentityTokenURL: ""

  # Optional: Custom audience for K8s JWT
  workloadIdentityAudience: ""
```

For the HTTP template authenticator (used with `k8s` and `spiffe` providers), additional fields are available:

```yaml
# HTTP Template Authenticator fields
workloadIdentityUsername: ""           # Username for credentials (e.g., robot account name)
workloadIdentityAuthHost: ""           # Override auth endpoint host
workloadIdentityPathTemplate: ""       # URL path template for auth request
workloadIdentityBodyTemplate: ""       # Request body template (for POST)
workloadIdentityMethod: ""             # HTTP method: GET or POST
workloadIdentityAuthType: ""           # Auth type: bearer, basic, or none
workloadIdentityParams: ""             # Custom parameters for templates (JSON)
workloadIdentityResponseTokenField: "" # JSON field to extract token from response
```

#### Service Account Annotations

Provider-specific configuration is read from service account annotations:

**AWS:**
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: argocd-project-production
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::123456789012:role/argocd-prod
```

**GCP:**
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: argocd-project-production
  annotations:
    iam.gke.io/gcp-service-account: argocd@project.iam.gserviceaccount.com
```

**Azure:**
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: argocd-project-production
  annotations:
    azure.workload.identity/client-id: "client-id-uuid"
    azure.workload.identity/tenant-id: "tenant-id-uuid"
```

### Detailed Examples

#### Example 1: AWS ECR with Multi-Project Setup

```yaml
# Service Account for production project
apiVersion: v1
kind: ServiceAccount
metadata:
  name: argocd-project-production
  namespace: argocd
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::123456789012:role/argocd-prod
---
# Repository pointing to production ECR
apiVersion: v1
kind: Secret
metadata:
  name: prod-ecr
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  type: helm
  url: oci://123456789012.dkr.ecr.us-west-2.amazonaws.com/prod-charts
  project: production
  workloadIdentityProvider: aws
---
# Application using the repository
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
spec:
  project: production
  source:
    repoURL: oci://123456789012.dkr.ecr.us-west-2.amazonaws.com/prod-charts
    chart: my-chart
    targetRevision: 1.0.0
```

#### Example 2: Quay Robot Federation with K8s OIDC

```yaml
# Service Account for project
apiVersion: v1
kind: ServiceAccount
metadata:
  name: argocd-project-default
  namespace: argocd
---
# Repository with K8s workload identity and HTTP template authenticator (Quay.io)
apiVersion: v1
kind: Secret
metadata:
  name: quay-repo
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  type: helm
  url: oci://quay.io/myorg/charts
  project: default
  workloadIdentityProvider: k8s
  workloadIdentityPathTemplate: "/oauth2/federation/robot/token"
  workloadIdentityMethod: GET
  workloadIdentityAuthType: basic
  workloadIdentityUsername: "myorg+argocd"
  workloadIdentityResponseTokenField: token
```

### Security Considerations

1. **Short-lived tokens**: All credentials obtained through workload identity are short-lived (typically 1 hour), reducing the impact of credential leakage.
2. **No stored secrets**: Long-lived credentials are never stored in Kubernetes secrets, eliminating a common attack vector.
3. **Cloud IAM integration**: Access control is enforced at the cloud IAM level, providing fine-grained permissions.
4. **Per-project isolation**: Each project can have its own identity with its own IAM permissions, preventing cross-project access.
5. **SPIFFE per-project authorization (special case) **: Authorization with SPIFFE is enforced through project-specific SPIFFE IDs.
  Each Argo CD project maps to a unique SPIFFE ID (e.g., `spiffe://trust-domain/argocd/project/production`), and registries
  are configured to trust and authorize specific SPIFFE IDs. This provides cryptographically-enforced per-project isolation
  without requiring elevated SPIRE privileges.
6. **TokenRequest API (everyone else) **: The implementation uses the Kubernetes TokenRequest API which provides bound
  service account tokens with configurable audiences and expiration.

### Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Cloud IAM misconfiguration grants excessive access | Documentation includes least-privilege IAM policy examples |
| SPIFFE ID misconfiguration grants wrong project access | Use consistent naming conventions; registry mappings should be reviewed |
| Token exchange failures cause sync failures | Clear error messages; fallback to existing credential mechanisms if configured |
| Complex setup for users unfamiliar with workload identity | Comprehensive documentation with step-by-step guides for each provider |

### Upgrade / Downgrade Strategy

**Upgrade:**
- The `workloadIdentityProvider` field defaults to empty, so existing repositories continue to work unchanged
- Users opt-in to workload identity by setting `workloadIdentityProvider` to their desired provider
- No migration required for existing deployments

**Downgrade:**
- Repositories with `workloadIdentityProvider` set will fail to authenticate on older versions
- Users must remove `workloadIdentityProvider` and provide static credentials before downgrading
- The field is ignored by older versions that don't recognize it

## Drawbacks

1. **Complexity**: Workload identity setup requires understanding cloud IAM concepts that may be unfamiliar to some users.
2. **Cloud provider dependency**: Each cloud provider has different setup requirements, increasing documentation and testing burden.
3. **SPIFFE/SPIRE adoption**: The SPIFFE provider requires SPIRE infrastructure which adds operational complexity.

## Alternatives

### Alternative 1: External Secrets Operator Integration

Use External Secrets Operator to sync credentials from cloud secret managers (AWS Secrets Manager, GCP Secret Manager, Azure Key Vault).

**Pros:**
- Works with existing Argo CD without code changes
- Credentials can be rotated in secret manager

**Cons:**
- Still stores credentials in Kubernetes secrets (even if synced)
- Requires additional operator installation

## References

### Cloud Provider Documentation
- [AWS IAM Roles for Service Accounts](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html)
- [GCP Workload Identity Federation](https://cloud.google.com/iam/docs/workload-identity-federation)
- [Azure Workload Identity](https://azure.github.io/azure-workload-identity/docs/)

### Standards and Specifications
- [SPIFFE/SPIRE](https://spiffe.io/)
- [Kubernetes TokenRequest API](https://kubernetes.io/docs/reference/kubernetes-api/authentication-resources/token-request-v1/)

### Related Projects and Implementations
- [Flux: SPIFFE/SPIRE Workload Identity Proposal](https://github.com/fluxcd/flux2/issues/5681) - Similar proposal for Flux with discussion of registry OIDC federation support
- [Zot: OIDC Workload Identity Federation](https://github.com/project-zot/zot/pull/3711) - Native OIDC bearer token authentication with CEL expression support