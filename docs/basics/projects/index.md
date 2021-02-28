# Projects

## Overview

The so-called *Projects* (or, *AppProject* alternatively) play a vital role in
the multi-tenancy and governance model of Argo CD. It is important to understand
how *Projects* work and how they impact *Applications* and permissions.

You can think of a *Project* as a way to group specific *Applications* together
to enforce a common set of governance rules and settings on those Applications,
with the settings being defined in the *Project*. For example, you can restrict
the kind of resources allowed in an *Application*, or restrict the *Application*
to source its manifests only from a certain repository, etc etc. Furthermore,
projects can issue *access tokens* scoped to applications within the given
project. These tokens can be used to access the Argo CD API for manipulation
of *Applications* associated with the project, and their permissions can be
configured using *Project* specific RBAC configuration.

*Projects* and Applications have a *1:n* relationship, that is, multiple
*Applications* can belong to the same *Project*, while each *Application* can
only belong to one *Project*. Furthermore, the association of an *Application*
to a *Project* is mandatory. It is not possible to have an *Application* that
is not associated to a *Project*.

An Argo CD *Project* is implemented as a Custom Resource `AppProject` in the
`argoproj.io/v1alpha1` API. 

All `AppProject` resources must exist in Argo CD's installation namespace
(`argocd` by default) in the cluster Argo CD is installed to in order to be
used by Argo CD. They cannot be installed in other clusters or namespaces.

!!! tip "The default project"
    Argo CD installs a default *Project* which permits everything and restricts
    nothing. The default *Project* is called, well, `default`.

