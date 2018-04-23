# Tracking and Deployment Strategies

An ArgoCD application spec provides several different means to track kubernetes resource manifests in git. This document describes the different techniques which can be used and the means of deploying those manifests to the target environment.

## Auto-Sync

In all tracking strategies described below, the application has the option to sync automatically. If auto-sync is configured, the new resources manifests will be applied automatically -- as soon as a difference is detected between the target state (git) and live state. If auto-sync is disabled, a manual sync will be needed using the Argo UI, CLI, or API.

## Branch Tracking

If a branch name is specified, ArgoCD will continually compare live state against the resource manifests defined at the tip of the specified branch.

To redeploy an application, a user makes changes to the manifests, and commit/push those the changes to the tracked branch, which will then be detected by ArgoCD controller. 

## Tag Tracking

If a tag is specified, the manifests at the specified git tag will be used to perform the sync comparison. This provides some advantages over branch tracking in that a tag is generally considered more stable, and less frequently updated, with some manual judgement of what constitutes a tag.

To redeploy an application, the user uses git to change the meaning of a tag by retagging it to a different commit SHA. ArgoCD will detect the new meaning of the tag when performing the comparison/sync.

## Commit Pinning

If a git commit SHA is specified, the application is effectively pinned to the manifests defined at the specified commit. This is the most restrictive of the techniques and is typically used to control production environments.

Since commit SHAs cannot change meaning, the only way to change the live state of an application which is pinned to a commit, is by updating the tracking revision in the application to a different commit containing the new manifests.

## Parameter Overrides

ArgoCD provides means to override the parameters of a ksonnet app. This gives some extra flexibility in having *some* parts of the k8s manifests determined dynamically. It also serves as an alternative way of redeploying an application by changing application parameters via ArgoCD, instead of making the changes to the manifests in git.

The following is an example of where this would be useful: A team maintains a "dev" environment, which needs to be continually updated with the latest version of their guestbook application after every build in the tip of master. To solve this, the ksonnet application would expose an parameter named `guestbookImage`, whose value used in the `dev` environment contains a placeholder value (e.g. `example/guestbook:replaceme`) intended to be set externally (outside of git) such as build systems. As part of the build pipeline, the parameter value of the `guestbookImage` would be continually updated to the freshly built image (e.g. `example/guestbook:abcd123`). A sync operation would result in application being redeployed with the new image.

The ArgoCD provides these operations conveniently via the CLI, or alternatively via the gRPC/REST API.
```
$ argocd app set guestbook -p guestbookImage:example/guestbook:abcd123
$ argocd app sync guestbook
```

Note that in all tracking strategies, any parameter overrides set in the application instance will be honored.

