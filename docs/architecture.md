
# Argo CD - Architectural Overview

![Argo CD Architecture](argocd_architecture.png)

## Components

### API Server
The API server is a gRPC/REST server which exposes the API consumed by the Web UI, CLI, and CI/CD 
systems. It has the following responsibilities:
* application management and status reporting
* invoking of application actions (e.g. manual sync, user-defined actions)
* repository and cluster credential management (stored as K8s secrets)
* authentication and RBAC enforcement, with eventual integration with external identity providers
* listener/forwarder for git webhook events

### Repository Server
The repository server is an internal service which maintains a local cache of the git repository
holding the application manifests. It is responsible for generating and returning the Kubernetes
manifests when provided the following inputs:
* repository URL
* git revision (commit, tag, branch)
* application path
* application environment

### Application Controller
The application controller is a Kubernetes controller which continuously monitors running
applications and compares the current, live state against the desired target state (as specified in
the git repo). It detects out-of-sync application state and optionally takes corrective action. It
is responsible for invoking any user-defined handlers (argo workflows) for Sync, OutOfSync events

### Application CRD (Custom Resource Definition)
The Application CRD is the Kubernetes resource object representing a deployed application instance
in an environment. It holds a reference to the desired target state (repo, revision, app, environment)
of which the application controller will enforce state against.
