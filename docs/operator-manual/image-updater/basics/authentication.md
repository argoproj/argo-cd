# Authentication in Argo CD Image Updater

There are several scenarios where Argo CD Image Updater needs to authenticate
to external systems in order to fulfil its duties. 

## <a name="auth-kubernetes"></a>Authentication to Kubernetes

If you are running Argo CD Image Updater as a Kubernetes workload in the
`kubernetes` API access mode (which is the default), it uses the token of a
ServiceAccount mounted to its pod to authenticate at the Kubernetes API.

The name of this ServiceAccount is `argocd-image-updater`, and it gets
created through the installation manifests in the installation namespace.
The ServiceAccount is accompanied with an appropriate Kubernetes RBAC
Role that holds the required permissions, and a RoleBinding to bind the
Role to the ServiceAccount.

In the default installation scenario, i.e. Argo CD Image Updater installed
to the `argocd` namespace, no further configuration has to done in order
for Argo CD Image Updater to access the Kubernetes API. If your Argo CD
installation is in a different namespace than `argocd`, you would have
to adapt the RoleBinding to bind to the ServiceAccount in the correct
namespace.

## <a name="auth-argocd"></a>Authentication to Argo CD

If you are using Argo CD Image Updater to connect to Argo CD via its API,
you will need to create credentials in Argo CD and provide them to the
Image Updater.

This usually involves the following steps:

1. Create a local user with `apiKey` capabilities in Argo CD
1. Generate an authentication token for that user
1. Create appropriate RBAC permissions for that user in Argo CD
1. Configure Argo CD Image Updater to connect to the Argo CD API endpoint
   instead of the Kubernetes API
1. Configure Argo CD Image Updater to use the credentials created in steps 1
   and 2 for authenticating at the Argo CD API

A complete walk-through can be found in the installation guide.

## <a name="auth-registries"></a>Authentication to container registries

If you are using private registries, or private repositories on public
registries, chances are that you will need to have Argo CD Image Updater use
credentials to access the registry.

Credentials can be configured either on a per registry basis or on a per image
basis. You can also use a mixed setup where credentials are configured in both
ways.

If credentials are configured on a per registry basis, all requests to that
registry will use these credentials, without requiring any further
configuration. Also, credentials configured for a registry will be cached
for a configurable time and don't have to be re-read for every authentication
requests. This can be useful especially for situations where you use tokens
with a limited life time that are generated externally, and don't want to
regenerate the token for each request.

However, you can override the credentials used for authentication to the same
registry on a per image basis, e.g. if you use repositories on the registry
that require a different set of credentials.

You can read more about
[configuration credentials at the registry level]()
and
[configuration credentials at the image level]()
for more details on how to configure either mechanisms.

### <a name="auth-registries-creds-sources"></a>Types of supported credential sources

Argo CD Image Updater can source credentials for accessing registries using the
following mechanisms:

* Secrets stored in Kubernetes. Argo CD Image Updater can either use a typical
  Docker pull secret, or a custom secret with credentials in a certain format.

* An environment variable available to Argo CD Image Updater (for example,
  an environment variable mounted to the pod from a secret)

* A script that, when executed, outputs the credentials to stdout.

The following sections describe the configuration format to be used for the
different types of credential sources.

### <a name="auth-registries-pull-secret"></a>Using a pull secret

A pull secret is a secret with a well-defined format, that can also be used
by Kubernetes' container engine to pull container images from registries.

These secrets typically have a field `.dockerconfigjson` in the `.data`
section, which holds a JSON string with the appropriate credentials.

In its default configuration, Argo CD Image Updater can only read secrets
from the same namespace where it is installed to.

Pull secrets can be referenced as follows in credentials configuration:

```
pullsecret:<namespace>/<secret_name>
```

Where `<namespace>` is the namespace the secret resides in, and `<secret_name>`
is the name of the Secret resource.

You can create a pull secret by using `kubectl`. The following command would
create a pull secret for Docker Hub named `dockerhub-secret` in the namespace
`argocd`:

```shell
kubectl create -n argocd secret docker-registry dockerhub-secret \
  --docker-username someuser \
  --docker-password s0m3p4ssw0rd \
  --docker-server "https://registry-1.docker.io"
```

This secret could then be referred to as

```
pullsecret:argocd/dockerhub-secret
```

### <a name="auth-registries-generic-secret"></a>Using a generic secret

Argo CD Image Updater can also retrieve credentials from a field in a generic
Secret. This may be useful if you have similar credentials for a different set
of registries, and don't want to maintain a unique pull-secret for each
registry.

The credentials can be stored in any field of the Secret's `.data` section,
but credentials must be stored in the format `<username>:<password>`.

To retrieve the credentials from a generic secret, you must specify the field
name along with the namespace and name of the secret like follows:

```
secret:<namespace>/<secret_name>#<field_name>
```

E.g. if you have stored your credentials in the field `creds` in the secret
`some-secret` in the namespace `argocd`, you would refer to it as

```
secret:argocd/some-secret#creds
```

### <a name="auth-registries-env"></a>Using an environment variable

Argo CD Image Updater can read credentials from an environment variable. This
can be useful for testing purposes (e.g. when using the command
`argocd-image-updater test` to test access to a registry), or if you have the
environment variable to use mounted from a secret to the `argocd-image-updater`
pod.

To retrieve credentials from an environment variable, you must specify the name
of the environment variable as follows:

```
env:<name_of_environment_variable>
```

The credentials passed via environment variables must be specified in the format
`<username>:<password>`, e.g. to store credentials in an environment variable
named `DOCKER_HUB_CREDS`:

```
DOCKER_HUB_CREDS=someuser:s0m3p4ssw0rd
```

And then, to use this environment variable, reference it as

```
env:DOCKER_HUB_CREDS
```

### <a name="auth-registries-script"></a>Using a script to generate credentials

Argo CD Image Updater supports using credentials that are generated by an
external script. In order to retrieve the credentials, Argo CD Image Updater
will execute a user configured script or executable, and parse its output.

Having a script generate the credentials can be useful if your registry does
require a short-lived token for authentication which is issued by a third
party system, possibly with this third party system requiring a different set
of credentials. A prominent example would be ECR on aws.

Referencing a script used to output the credentials is done as follows:

```
ext:<full_path_to_script>
```

When executing the script, Argo CD Image Updater does not pass any arguments
to it.

The executed script is expected to output exactly one line to stdout, which
holds the credentials used for accessing the registry in the format
`<username>:<password>`. Please note that the output should contain a newline
character. For example, the most simple form of such a script would be:

```bash
#!/bin/sh

echo "someuser:s0mep4ssw0rd"
```

When executing on Kubernetes, the script to be executed must exist in the
Image Updater container's file system. You can either mount the script from
a config map, or use an init container to copy it. Make sure that the script
is executable.

For example, if above script would exist at `/usr/local/bin/creds.sh`, it
would be referenced as

```
ext:/usr/local/bin/creds.sh
```

Please keep in mind that executing scripts to retrieve credentials can become
expensive. If possible, use this method only on a per-registry level with
proper caching. Read more about this in the
[Registry configuration]()
section of the docs.

* A typical pull secret, i.e. a secret containing a `.dockerconfigjson` field
  which holds a Docker client configuration with auth information in JSON
  format. This kind of secret is specified using the notation
  `pullsecret:<namespace>/<secret_name>`

* A custom secret, which has the credentials stored in a configurable field in
  the format `<username>:<password>`. This kind of secret is specified using
  the notation `secret:<namespace>/<secret_name>#<field_in_secret>`

* An environment variable which holds the credentials in the format
  `<username>:<password>`. This kind of secret is specified using the notation
  `env:<env_var_name>`.

* A script that outputs credentials on a single line to stdout, in the format
  `<username:password>`. This can be used to support external authentication
  mechanisms. You can specify this kind of secret in the notation
  `ext:/path/to/script`. Please note that the script must be referenced as
  absolute path, and must be executable (i.e. have the `+x` bit set). You
  can add scripts to `argocd-image-updater` by using an init container.

