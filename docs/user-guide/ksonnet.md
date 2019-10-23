# Ksonnet

!!! tip Warning "Ksonnet is defunct and no longer supported."

## Environments
Ksonnet has a first class concept of an "environment." To create an application from a ksonnet
app directory, an environment must be specified. For example, the following command creates the
"guestbook-default" app, which points to the `default` environment:

```bash
argocd app create guestbook-default --repo https://github.com/argoproj/argocd-example-apps.git --path guestbook --env default
```

## Parameters
Ksonnet parameters all belong to a component. For example, the following are the parameters
available in the guestbook app, all of which belong to the `guestbook-ui` component:

```bash
$ ks param list
COMPONENT    PARAM         VALUE
=========    =====         =====
guestbook-ui containerPort 80
guestbook-ui image         "gcr.io/heptio-images/ks-guestbook-demo:0.1"
guestbook-ui name          "guestbook-ui"
guestbook-ui replicas      1
guestbook-ui servicePort   80
guestbook-ui type          "LoadBalancer"
```

When overriding ksonnet parameters in Argo CD, the component name should also be specified in the
`argocd app set` command, in the form of `-p COMPONENT=PARAM=VALUE`. For example:

```bash
argocd app set guestbook-default -p guestbook-ui=image=gcr.io/heptio-images/ks-guestbook-demo:0.1
```

## Build Environment

We do not support the [standard build environment](build-environment.md) for Ksonnet.
