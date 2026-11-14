# Automation from CI Pipelines

Argo CD follows the GitOps model of deployment, where desired configuration changes are first
pushed to Git, and the cluster state then syncs to the desired state in git. This is a departure
from imperative pipelines which do not traditionally use Git repositories to hold application
config.

To push new container images to a cluster managed by Argo CD, the following workflow (or 
variations) might be used:

## Build And Publish A New Container Image

```bash
docker build -t mycompany/guestbook:v2.0 .
docker push mycompany/guestbook:v2.0
```

## Update The Local Manifests Using Your Preferred Templating Tool, And Push The Changes To Git

> [!TIP]
> The use of a different Git repository to hold your Kubernetes manifests (separate from
> your application source code), is highly recommended. See [best practices](best_practices.md)
> for further rationale.

```bash
git clone https://github.com/mycompany/guestbook-config.git
cd guestbook-config

# kustomize
kustomize edit set image mycompany/guestbook:v2.0

# plain yaml
kubectl patch --local -f config-deployment.yaml -p '{"spec":{"template":{"spec":{"containers":[{"name":"guestbook","image":"mycompany/guestbook:v2.0"}]}}}}' -o yaml > config-deployment.yaml

git commit -am "Update guestbook to v2.0"
git push
```

## Authentication in CI Pipelines

CI pipelines run without a browser, so the usual `argocd login --sso` flow doesn't apply.
Set `ARGOCD_AUTH_TOKEN` (or pass `--auth-token`) with a token obtained by one of these methods:

- Project role token — `argocd proj role create-token`. Scoped to a single AppProject; no local user account needed.
- Local user API token — `argocd account generate-token` on a [local user](../operator-manual/user-management/index.md#local-usersaccounts) with the `apiKey` capability. Simplest option if local service accounts are permitted.
- Dex Token Exchange — your CI platform's OIDC token (GitHub Actions, GitLab CI, etc.) is exchanged for a Dex token with a single `curl` call. Works headlessly without requiring local accounts.
- External OIDC ID token — when Argo CD uses an external OIDC provider (no Dex), pass the CI platform's ID token directly if the provider is the same one Argo CD is configured to use.
- `--core` mode — `argocd login --core` bypasses Argo CD auth entirely and uses kubeconfig or in-cluster credentials instead. No `ARGOCD_AUTH_TOKEN` needed.

See [CI/CD Pipeline Authentication](../operator-manual/user-management/index.md#cicd-pipeline-authentication) for configuration details and examples.

## Synchronize The App (Optional)

For convenience, the argocd CLI can be downloaded directly from the API server. This is
useful so that the CLI used in the CI pipeline is always kept in-sync and uses argocd binary
that is always compatible with the Argo CD API server.

```bash
export ARGOCD_SERVER=argocd.example.com
export ARGOCD_AUTH_TOKEN=<token — see Authentication in CI Pipelines above>
curl -sSL -o /usr/local/bin/argocd https://${ARGOCD_SERVER}/download/argocd-linux-amd64
argocd app sync guestbook
argocd app wait guestbook
```

If [automated synchronization](auto_sync.md) is configured for the application, this step is
unnecessary. The controller will automatically detect the new config (fast tracked using a
[webhook](../operator-manual/webhook.md), or polled at least every 3 minutes by default), and automatically sync the new manifests.
