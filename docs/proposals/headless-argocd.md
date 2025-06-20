---
title: Neat-enhancement-idea
authors:
- "@alexmt"
  sponsors:
- TBD
  reviewers:
- "@jessesuen"
- TBD
  approvers:
- "@jessesuen"
- TBD

creation-date: 2020-05-01
last-updated: 2020-05-01
---

# Neat Enhancement Idea

Support "disabling" multi-tenancy features by introducing Headless Argo CD.

## Summary

There are two main group of GitOps users:

* Application developers - engineers who leverages Kubernetes to run applications.
* Cluster administrators - engineers who manage and support Kubernetes clusters for the organization.

Argo CD is a perfect fit for application developers thanks to its multi-tenancy features. Instead of running a separate Argo CD instance for
each team, it is possible to run on the instance and leverage features like SSO, RBAC, and Web user interface. However, this is not the case
for cluster administrators. Administrators prefer to rely on Kubernetes RBAC and view SSO and Argo CD RBAC as an obstacle and security threat.
SSO, RBAC, and UI/API are totally optional and can be disabled but it requires additional configuration and learning.

## Motivation

It is proposed to introduce officially supported **Headless Argo CD** that encapsulates changes required to disable multi-tenancy features
and provide a seamless experience for cluster administrators (or any other user who don't need multi-tenancy).

### Goals

The goals of "Headless Argo CD" are:

#### Provide an easy way to deploy Argo CD without API/UI

The end-user should be able to install required components using a single `kubectl apply` command without following any additional instructions.

#### Provide an easy way to use and manage Headless Argo CD

The `Headless Argo CD` should provide a simple way to view and manage Argo CD applications using CLI/UI. The access control should be enforced by
Kubernetes RBAC only.

#### Easy transition from Headless to non-Headless Argo CD

It is a common case when the Argo CD adopter wants to start small and then expand Argo CD to the whole organization. It should be easy
to "upgrade" headless to full Argo CD installation.

### Non-Goals

#### Not modified Argo CD

The `Headless Argo CD` is not modified Argo CD. It is Argo CD distribution that missing UI/API and CLI that provides commands for Argo CD admin.

#### Not deprecating existing operational methods

The `Headless Argo CD` is not intended to deprecate any of the existing operational methods.

## Proposal

#### Headless Installation Manifests

In order to simplify installation of Argo CD without API we need introduce `headless/install.yaml` in [manifests](../../manifests) directory.
The installation manifests should include only non HA controller, repo-server, Redis components, and RBAC.

#### Headless CLI

Without the API server, users won't be able to take advantage of Argo CD UI and `argocd` CLI so the user experience won't be complete. To fill that gap
we need to change the `argocd` CLI that and support talking directly to Kubernetes without requiring Argo CD API Server. The [argo-cd#6361](https://github.com/argoproj/argo-cd/pull/6361)
demonstrates required changes:

* Adds `--headless` flag to `argocd` commands
* If the `--headless` flag is set to true then pre-run function that starts "local" Argo CD API server and points CLI to locally running instance
* Finally on-demand port-forwards to Redis and repo server.

The user should be able to store `--headless` flag in config in order to avoid specifying the flag for every command. It is proposed to use `argocd login --headless` to generate
"headless" config.

#### Local UI

In addition to exposing CLI commands the PR introduces `argocd admin dashboard` command. The new command starts API server locally and exposes Argo CD UI locally.
In order to make this possible the static assets have been embedded into Argo CD binary.

### Merge Argo CD Util

The potential users of "headless" mode will benefit from `argocd-util` commands. The experience won't be smooth since they will need to switch back and forth
between `argocd` and `argocd-util`. Given that we still have not finalized how users are supposed to get `argocd-util` binary (https://github.com/argoproj/argo-cd/issues/5307)
it is proposed to deprecate `argocd-util` and merge in into `argocd` CLI under admin subcommand:

```
argocd admin app generate-spec guestbook --repo https://github.com/argoproj/argocd-example-apps
```

### Use cases

Add a list of detailed use cases this enhancement intends to take care of.

## Use case 1:

As an Argo CD administrator, I would like to manage cluster resources using Argo CD without exposing API/UI outside of the cluster.

## Use case 2:

As an Argo CD administrator, I would like to use Argo CD CLI commands and user interface to manage Argo CD applications/settings using only `kubeconf` file and without Argo CD API access.

### Security Considerations

The Headless CLI/UI disables built-in Argo CD authentication and relies only on Kubernetes RBAC. So if the user will be able to make the same change using Headless CLI as using kubectl.

### Risks and Mitigations

TBD

### Upgrade / Downgrade Strategy

Switching to and from Argo CD Headless does not modify any persistent data or settings. So upgrade/downgrade should be seamless by just applying the right manifest file.

## Drawbacks

* Embedding static resources into the binary increases it's size by ~20 mb. The image size is the same.

## Alternatives

* Re-invent GitOps Agent CLI experience and don't re-use Argo CD.