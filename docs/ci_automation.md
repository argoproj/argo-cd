# Automation from CI Pipelines

Argo CD follows the GitOps model of deployment, where desired configuration changes are first
pushed to git, and the cluster state then syncs to the desired state in git. This is a departure
from imperative pipelines which do not traditionally use git repositories to hold application
config.

To push new container images into to a cluster managed by Argo CD, the following workflow (or 
variations), might be used:


1. Build and publish a new container image

    ```
    docker build -t mycompany/guestbook:v2.0 .
    docker push mycompany/guestbook:v2.0
    ```

2. Update the local manifests using your preferred templating tool, and push the changes to git.

    NOTE: the use of a different git repository to hold your kubernetes manifests (separate from
    your application source code), is highly recommended. See [best practices](best_practices.md)
    for further rationale.

    ```
    git clone https://github.com/mycompany/guestbook-config.git
    cd guestbook-config

    # kustomize
    kustomize edit set imagetag mycompany/guestbook:v2.0

    # ksonnet
    ks param set guestbook image mycompany/guestbook:v2.0

    # plain yaml
    kubectl patch --local -f config-deployment.yaml -p '{"spec":{"template":{"spec":{"containers":[{"name":"guestbook","image":"mycompany/guestbook:v2.0"}]}}}}' -o yaml

    git add . -m "Update guestbook to v2.0"
    git push
    ```

3. Synchronize the app (Optional)

    For convenience, the argocd CLI can be downloaded directly from the API server. This is
    useful so that the CLI used in the CI pipeline is always kept in-sync and uses argocd binary
    that is always compatible with the Argo CD API server.

    ```
    export ARGOCD_SERVER=argocd.mycompany.com
    export ARGOCD_AUTH_TOKEN=<JWT token generated from project>
    curl -sSL -o /usr/local/bin/argocd https://${ARGOCD_SERVER}/download/argocd-linux-amd64
    argocd app sync guestbook
    argocd app wait guestbook
    ```

    If [automated synchronization](auto_sync.md) is configured for the application, this step is
    unnecessary. The controller will automatically detect the new config (fast tracked using a
    [webhook](webhook.md), or polled every 3 minutes), and automatically sync the new manifests.
