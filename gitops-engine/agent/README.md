# GitOps Agent

The GitOps Agent leverages the GitOps Engine and provides access to many engine features via a simple CLI interface.
The agent provides the same set of core features as Argo CD including basic reconciliation, syncing as well as sync hooks and sync waves.

The main difference is that the agent is syncing one Git repository into the same cluster where it is installed.

## Quick Start

By default, the agent is configured to use manifests from [guestbook](https://github.com/argoproj/argocd-example-apps/tree/master/guestbook)
directory in https://github.com/argoproj/argocd-example-apps repository.

The agent supports two modes:

* namespaced mode - agent manages the same namespace where it is installed
* full cluster mode - agent manages the whole cluster

### Namespaced Mode

Install the agent with the default settings using the command below. Done!

```bash
kubectl apply -f https://raw.githubusercontent.com/argoproj/gitops-engine/master/agent/manifests/install-namespaced.yaml 
kubectl rollout status deploy/gitops-agent
```

The agent logs:

```bash
kubectl logs -f deploy/gitops-agent gitops-agent
```

Find the guestbook deployment in the current K8S namespace:

```bash
kubectl get deployment
```

### Cluster Mode

The cluster mode grants full cluster access to the GitOps Agent. Use the following command to install an agent into the
`gitops-agent` namespace and use it to manage resources in any cluster namespace.

> Note. In cluster mode agents gets **full** cluster access.
> See [gitops-agent-cluster-role.yaml](./manifests/cluster-install/gitops-agent-cluster-role.yaml) definition for more information. 

```bash
kubectl create ns gitops-agent
kubectl apply -f https://raw.githubusercontent.com/argoproj/gitops-engine/master/agent/manifests/install.yaml -n gitops-agent
```

### Customize Git Repository

The agent runs [git-sync](https://github.com/kubernetes/git-sync) as a sidecar container to access the repository.
Update the container env [variables](https://github.com/kubernetes/git-sync#parameters) to change the repository.

### Demo Recording

[![asciicast](https://asciinema.org/a/FWbvVAiSsiI87wQx2TJbRMlxN.svg)](https://asciinema.org/a/FWbvVAiSsiI87wQx2TJbRMlxN)


### Profiling

Using env variables to enable profiling mode, the agent can be started with the following envs:

```bash
export GITOPS_ENGINE_PROFILE=web
# optional, default pprofile address is 127.0.0.1:6060
export GITOPS_ENGINE_PROFILE_HOST=127.0.0.1
export GITOPS_ENGINE_PROFILE_PORT=6060
```

And then you can open profile in the browser(or using [pprof](https://github.com/google/pprof) cmd to generate diagrams):

- http://127.0.0.1:6060/debug/pprof/goroutine?debug=2
- http://127.0.0.1:6060/debug/pprof/mutex?debug=2
