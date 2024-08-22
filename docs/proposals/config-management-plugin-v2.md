---
title: Config-Management-Plugin-Enhancement
authors:
  - "@kshamajain99" # Authors' github accounts here.
sponsors:
  - TBD        # List all intereste parties here.
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2021-03-29
last-updated: 2021-03-29
---

# Config Management Plugin Enhancement

We want to enhance config management plugin in order to improve Argo CD operator and end-user experience 
for using additional tools such as cdk8s, Tanka, jkcfg, QBEC, Dhall, pulumi, etc. 

## Summary

Currently, Argo CD provides first-class support for Helm, Kustomize and Jsonnet/YAML. The support includes:

- Bundled binaries (maintainers periodically upgrade binaries)
- An ability to override parameters using UI/CLI
- The applications are discovered in Git repository and auto-suggested during application creation in UI
- Performance optimizations. Argo CD "knows" when it is safe to generate manifests concurrently and takes advantage of it.

We want to enhance the configuration management plugin so that it can provide similar first-class support for additional 
tools such as cdk8s, Tanka, jkcfg, QBEC, Dhall, pulumi, etc.

## Motivation

The config management plugin feature should be improved to provide the same level of user experience as 
for the natively supported tools to the additional tools such as  cdk8s, Tanka, jkcfg, QBEC, Dhall, pulumi, etc., 
including Argo CD operators as well as end-user experience.

### Goals

The goals for config management plugin enhancement are,

#### Improve Installation Experience
The current Config Management plugin installation experience requires two changes:

- An entry in configManagementPlugins in the Argo CD configmap (i.e.  argocd-cm)
- Either an init container with a volume mount that adds a new binary into Argo CD repo server pod, or a rebuild of the argocd image, which contains the necessary tooling

The problem with this approach is that the process is error-prone, manual, and requires learning from each and every Argo CD administrator. 

The goal is to make additional tools easily accessible for installation to Argo CD operators.

#### Provide Discovery (Auto-selection of Tool)
For Argo CD’s natively supported config management plugins (Helm, Kustomize, Jsonnet), Argo CD auto-detects 
and selects the appropriate tool given only the path in the Git repository. 
This selection is based on the recognition of well-known files in the directory (e.g. Chart.yaml, kustomization.yaml, etc...). 

Currently, unlike natively supported tools, when a plugin is used, a user needs to explicitly specify the plugin 
that should be used to render the manifests. As part of the improvements to config management plugins, 

We want to provide the same ability to auto-select the plugin based on recognized files in the path of the git repository.

#### Parameters support in UI/CLI
Currently, configuration management plugins allow specifying only a list of environment variables via UI/CLI. 

We want to extend its functionality to provide a similar experience as for existing natively supported tools 
to additional config management tools. 

### Non-Goals

- We aren't planning on changing the existing support for native plugins as of now. 

## Proposal

We have drafted the solution to the problem statement as **running configuration management plugin tools as sidecar in the argocd-repo-server**. 

All it means that Argo CD Config Management Plugin 2.0 will be,

- A user-supplied container image with all the necessary tooling installed in it. 
- It will run as a sidecar in the repo server deployment and will have shared access to the git repositories.
- It will contain a CMP YAML specification file describing how to render manifests.
- Its entrypoint will be a lightweight CMP API server that receives requests by the main repo-server to render manifests, 
based on the CMP specification file.

This mechanism will provide the following benefits over the existing solution,

- Plugin owners control their execution environment, packaging whatever dependent binaries required.
- An  Argo CD user who wants to use additional config management tools does not have to go through the hassle of building 
a customized argocd-repo-server in order to install required dependencies. 
- The plugin image will be running in a container separate from the main repo-server.

### Use cases

- UC1: As an Argo CD user, I would like to use first-class support provided for additional tools to generate and manage deployable kubernetes manifests
- UC2: As an Argo CD operator, I want to have smooth experience while installing additional tools such as  cdk8s, Tanka, jkcfg, QBEC, Dhall, pulumi, etc.
- UC3: As a plugin owner, I want to have some control over the execution environment as I want to package whatever dependent binaries required. 

### Implementation Details

Config Management Plugin v2.0 implementation and experience will be as,

#### Installation

To install a plugin, an operator will simply patch argocd-repo-server to run config management plugin container as a sidecar, 
with argocd-cmp-server as it’s entrypoint. Operator can use either off-the-shelf or custom built plugin image as sidecar image. 

```bash
# A plugin is a container image which runs as a sidecar, with the execution environment
# necessary to render manifests. To install a plugin, 
containers:
- name: cdk8s
  command: [/var/run/argocd/argocd-cmp-server]
  image: docker.ui/cdk8s/cdk8s:latest
  volumeMounts:
  - mountPath: /var/run/argocd
    name: var-files
```

The argocd-cmp-server binary will be populated inside the plugin container via an init container in the argocd-repo-server, 
which will pre-populate a volume shared between plugins and the repo-server.

```bash
# An init container will copy the argocd static binary into the shared volume
# so that the CMP server can become the entrypoint
initContainers:
- command:
  - cp
  - -n
  - /usr/local/bin/argocd
  - /var/run/argocd/argocd-cmp-server
  image: quay.io/argoproj/argocd:latest
  name: copyutil
  volumeMounts:
  - mountPath: /var/run/argocd
    name: var-files
 
# var-files is a shared volume between repo-server and cmp-server which holds:
# 1) socket files that repo-server uses to communicate to each plugin
# 2) git repositories cloned by repo-server
volumes:
- emptyDir: {}
  name: var-files
```

#### Configuration

Plugins will be configured via a ConfigManagementPlugin manifest located inside the plugin container, placed at a 
well-known location (e.g. /home/argocd/plugins/plugin.yaml). Argo CD is agnostic to the mechanism of how the plugin.yaml would be placed, 
but various options can be used on how to place this file, including: 

- Baking the file into the plugin image as part of docker build
- Volume mapping the file through a configmap.

Note that, while the ConfigManagementPlugin looks like a Kubernetes object, it is not actually a custom resource. 
It only follows kubernetes-style spec conventions.

```bash
# metadata file is in the root and shell executor knows about it
apiVersion: argoproj.io/v1alpha1
kind: ConfigManagementPlugin
metadata:
  name: cdk8s
spec:
  version: v1.0
  init:
    command: [cdk8s, init]
  generate:
    command: [sh, -c, "cdk8s synth && cat dist/*.yaml"]
  discovery:
    find:
    - command: [find . -name main.ts]
      glob: "**/*/main.ts"
    check:
    - command: [-f ./main.ts]
      glob: "main.ts"
```

#### Config Management Plugin API Server (cmp-server)
The Config Management Plugin API Server (cmp-server) will be a new Argo CD component whose sole responsibility will be 
to execute `generate` commands inside the plugin environment (the sidecar container), at the request of the repo-server.

The cmp-server will expose the following APIs to the repo-server,

- GenerateManifests(path) - returns YAML output using plugin tooling
- IsSupported(path) - returns whether or not the given path is supported by the plugin

At startup, cmp-server looks at the /home/argocd/cmp-server/plugin.yaml ConfigManagementPlugin specification file to understand how to perform the requests.

#### Registration & Communication
The repo-server needs to understand what all plugins are available to render manifests. To do this, the cmp-server 
sidecars will register themselves as available plugins to the argocd-repo-server by populating named socket files in the 
shared volume between repo-server and cmp-server. e.g.:

```bash
/home/argocd/plugins/
                        cdk8s.sock
                        jkcfg.sock
                        pulumi.sock
```

The name of the socket file will indicate the plugin name. To discover the available plugins, the repo-server will list 
the shared plugins directory to discover the available plugins.

To communicate with a plugin, the repo-server will simply need to connect to the socket and make gRPC calls against the 
cmp-server listening on the other side. 

#### Discovery (Auto-selection of Tool)

- The plugin discovery will run in the main repo-server container.
- Argo CD repo-server lists the shared plugins directory and runs `discover` command from the specification file, 
whichever plugin provides a positive response first will be selected.

#### Versioning
There will be one sidecar container per version. Hence, for two different versions users will have to configure two different sidecars.

### Security Considerations

The use of the plugin as sidecars separate from the repo-server is already a security improvement over the current v1.8 
config management plugin mechanism, since the plugin tooling will no longer have access to the files of the argocd-repo-server image. 
However additional improvements can be made to increase security.

### Risks and Mitigations

One issue is that currently when repositories are cloned, the repo is cloned using the same UID of the repo-server user, 
and so all repository files are created using that UID. This means that a command which executes in the git repository path, 
could traverse upwards and see/write files which are outside of the repository tree.

One proposal to prevent out-of-tree access to files, is that each git repository could be cloned with unique UIDs, 
different from the repo-server’s UID. When the cmp-server executes the tooling command to generate manifests, 
the command could be executed using the UID of the git repository files. e.g.:

```
cmd := exec.Command(command, args...)
cmd.SysProcAttr = &syscall.SysProcAttr{}
cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uid, Gid: gid}
```

This would ensure that the command could not read or write anything out-of-tree from the repository directory.

### Upgrade / Downgrade Strategy

The argocd-repo-server manifest will change in order to populate the argocd-cmp-server binary inside the plugin container 
via an init container.

```bash
# An init container will copy the argocd static binary into the shared volume
# so that the CMP server can become the entrypoint
initContainers:
- command:
  - cp
  - -n
  - /usr/local/bin/argocd
  - /var/run/argocd/argocd-cmp-server
  image: quay.io/argoproj/argocd:latest
  name: copyutil
  volumeMounts:
  - mountPath: /var/run/argocd
    name: var-files
 
# var-files is a shared volume between repo-server and cmp-server which holds:
# 1) socket files that repo-server uses to communicate to each plugin
# 2) git repositories cloned by repo-server
volumes:
- emptyDir: {}
  name: var-files
```
  
After upgrading to CMP v2, an Argo CD operator will have to make following changes,

- In order to install a plugin, an Argo CD operator will simply have to patch argocd-repo-server 
to run config management plugin container as a sidecar, with argocd-cmp-server as it’s entrypoint:

    ```bash
    # A plugin is a container image which runs as a sidecar, with the execution environment
    # necessary to render manifests. To install a plugin, 
    containers:
    - name: cdk8s
      command: [/var/run/argocd/argocd-cmp-server]
      image: docker.ui/cdk8s/cdk8s:latest
      volumeMounts:
      - mountPath: /var/run/argocd
        name: var-files
    ```

- Plugins will be configured via a ConfigManagementPlugin manifest located inside the plugin container, placed at a 
well-known location (e.g. /plugin.yaml). Argo CD is agnostic to the mechanism of how the plugin.yaml would be placed, 
but various options can be used on how to place this file, including: 
    - Baking the file into the plugin image as part of docker build
    - Volume mapping the file through a configmap.

(For more details please refer to [implementation details](#configuration))

## Drawbacks

There aren't any major drawbacks to this proposal. Also, the advantages supersede the minor learning curve of the new way of managing plugins.

However following are few minor drawbacks,

- With addition of plugin.yaml, there will be more yamls to manage
- Operators need to be aware of the modified Kubernetes manifests in the subsequent version.
- The format of the CMP manifest is a new "contract" that would need to adhere the usual Argo CD compatibility promises in future.

## Alternatives

1. ConfigManagementPlugin as CRD. Have a CR which the human operator creates:

    ```bash
    apiVersion: argoproj.io/v1alpha1
    kind: ConfigManagementPlugin
    metadata:
      name: cdk8s
    spec:
      name: cdk8s
      image: docker.ui/cdk8s/cdk8s:latest
      version: v1.0
      init:
        command: [cdk8s, init]
      generate:
        command: [sh, -c, "cdk8s synth && cat dist/*.yaml"]
        discovery:
        find:
        - command: [find . -name main.ts]
          glob: "**/*/main.ts"
          check:
        - command: [-f ./main.ts]
          glob: "main.ts"
    ```

2. Something magically patches the relevant manifest to add the sidecar.
