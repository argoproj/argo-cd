# Overview

!!!note

    This section contains information on upgrading Argo CD. Before upgrading please make sure to read details about
    the breaking changes between Argo CD versions.

Argo CD uses the semver versioning and ensures that following rules:

* The patch release does not introduce any breaking changes. So if you are upgrading from v1.5.1 to v1.5.3
 there should be no special instructions to follow.
* The minor release might introduce minor changes with a workaround. If you are upgrading from v1.3.0 to v1.5.2
please make sure to check upgrading details in  both [v1.3 to v1.4](./1.3-1.4.md)  and  [v1.4 to v1.5](./1.4-1.5.md)
 upgrading instructions.
 * The major release introduces backward incompatible behavior changes. It is recommended to take a backup of
 Argo CD settings using disaster recovery [guide](../disaster_recovery.md).

After reading the relevant notes about possible breaking changes introduced in Argo CD version use the following
command to upgrade Argo CD. Make sure to replace `<version>` with the required version number:

**Non-HA**:

```bash
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/<version>/manifests/install.yaml
```

**HA**:
```bash
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/<version>/manifests/ha/install.yaml
```

!!! warning

    Even though some releases require only image change it is still recommended to apply whole manifests set.
    Manifest changes might include important parameter modifications and applying the whole set will protect you from
    introducing misconfiguration.

<hr/>

* [v2.1 to v2.2](./2.1-2.2.md)
* [v2.0 to v2.1](./2.0-2.1.md)
* [v1.8 to v2.0](./1.8-2.0.md)
* [v1.7 to v1.8](./1.7-1.8.md)
* [v1.6 to v1.7](./1.6-1.7.md)
* [v1.5 to v1.6](./1.5-1.6.md)
* [v1.4 to v1.5](./1.4-1.5.md)
* [v1.3 to v1.4](./1.3-1.4.md)
* [v1.2 to v1.3](./1.2-1.3.md)
* [v1.1 to v1.2](./1.1-1.2.md)
* [v1.0 to v1.1](./1.0-1.1.md)
