# Application Source

An *Application Source* tells Argo CD where the manifests for the application
resources reside, how to generate them and which version or revision of the
source to use. The application source is defined in `.spec.source` part of the
*Application* Custom Resource.

Argo CD currently supports two types of sources to track application resources
from: Git and Helm repositories. Depending on the type of source you are
configuring for the application, specific configuration has to be performed.

## Source Repository

The `.spec.source.repoURL` field defines the repository where the resource
manifests are stored. It must be an URL that points to either a Git or a Helm
repository. For more details about supported URLs, authentication, etc. please
read the
[repositories documentation](../repositories/).

### Manifests from a Git repository

If you source manifests from a Git repository, you can use all the generators
that are supported by Argo CD to render the manifests before they are being
reconciled into your cluster. For list of available generator tools, please
have a look at the
[tools documentation](../../tools/)

#### Path within repository

The `.spec.source.path` defines the path within the repository that contains
your resources manifests. The path must be specified relative to the repository
root. If you want to use the top-level directory in your repository, use the
value `.` as path.

The corresponding parameter for `argocd app create` to specify the path is
`--path`.

#### Target revision

The `.spec.source.targetRevision` defines the Git target revision to track.
This can take various formats, please refer to the
[Tracking strategies documentation](../../syncing/tracking.md)
for more information.

The corresponding parameter for `argocd app create` to specify the target
revision is `--targetRevision`.

### Manifests from a Helm repository

If you source your manifests from a Helm repository, only Helm can be used to
render the manifests, obviously.

#### Specifying the name of the Chart

Instead of using `.spec.source.path`, you need to set `.spec.source.chart` to
the name of the Helm chart to be used.

#### Specifying the version of the Chart

Also, `.spec.source.targetRevision` specifies the version of the Helm chart to
use instead of a Git revision.

For more information, refer to the
[Helm tooling documentation](../../tools/helm.md).

## Source specific configuration

Depending on the tool you use to render the manifests for a given *Application*,
additional configuration can or must be given. These can be simple options to
recursively consider all of the manifests found in directories below
`.spec.source.path`, a directive to use a pinned version of a given tool or more
complex settings, like
[parameter overrides](../../tools/)

Please refer to the
[tool specific documenation](../../tools/)
for more information about the possible configuration options for each of the
supported tools.
