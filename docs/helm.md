# Helm

Helm applications can be created in two ways. As standard application based off a Git repo, or as a specific version of a chart from a Helm repo.

## Creating An App From A Chart From a Helm Repository

If you want to create an appusing Helm repo, you must create a Helm repo in ArgoCD (see [Declaritve Set-up](declarative-setup.md#helm-repositories), then:

~~~bash
argocd app create wordpress \
    --repo https://kubernetes-charts.storage.googleapis.com \
    --path wordpress \
    --revision 5.4.0 \
    --dest-namespace default \
    --dest-server https://localhost:6443
~~~

## Notes

Unlike Git repos, Helm repos do not support directory listings. So this mean they do not support

* Auto-completion of paths when creating apps.
* Automatic syncing.

Additionally:

* The field `source/repoUrl` is inferred as a Helm apps only if it is set as a Helm repo in the settings. 