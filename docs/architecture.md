
# Argo CD - Architectural Overview

![Argo CD Architecture](argocd_architecture.png)

## Components

### API Server
The API server is a gRPC/REST server which exposes the API consumed by the Web UI, CLI, and CI/CD 
systems. It has the following responsibilities:
* application management and status reporting
* invoking of application operations (e.g. sync, rollback, user-defined actions)
* repository and cluster credential management (stored as K8s secrets)
* authentication and auth delegation to external identity providers
* RBAC enforcement
* listener/forwarder for git webhook events

### Repository Server
The repository server is an internal service which maintains a local cache of the git repository
holding the application manifests. It is responsible for generating and returning the Kubernetes
manifests when provided the following inputs:
* repository URL
* git revision (commit, tag, branch)
* application path
* template specific settings: parameters, ksonnet environments, helm values.yaml

### Application Controller
The application controller is a Kubernetes controller which continuously monitors running
applications and compares the current, live state against the desired target state (as specified in
the git repo). It detects `OutOfSync` application state and optionally takes corrective action. It
is responsible for invoking any user-defined hooks for lifcecycle events (PreSync, Sync, PostSync)

### Application CRD (Custom Resource Definition)
The Application CRD is the Kubernetes resource object representing a deployed application instance
in an environment. It is defined by two key pieces of information:
* `source` reference to the desired state in git (repository, revision, path, environment)
* `destination` reference to the target cluster and namespace.

An example spec is as follows:

```
spec:
  project: default
  source:
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    targetRevision: HEAD
    path: guestbook
    environment: default
  destination:
    server: https://kubernetes.default.svc
    namespace: default
```
