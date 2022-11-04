# Configuration of Container Registries

## <a name="supported-registries"></a>Supported registries

Argo CD Image Updater comes with out of the box support for most container
registries that implement the Docker registry v2 API.

It has been successfully tested against the following popular registries:

* Docker Hub (`docker.io`)
* Docker Registry v2 reference implementation (on-premise)
* Red Hat Quay (`quay.io` and on-premise)
* JFrog Artifactory (`jfrog.io` and on-premise)
* GitHub Container Registry (`ghcr.io`)
* GitHub Packages Registry (`docker.pkg.github.com`)
* GitLab Container Registry (`registry.gitlab.com`)
* Google Container Registry (`gcr.io`)

Chances are, that it will work out of the box for other registries as well.

If you have a registry that doesn't work with Argo CD Image Updater, please
let us know.

## <a name="custom-registry-when"></a>When to configure a custom registry?

Generally, there is no need to configure custom registries. Image Updater will
infer the registry from the image slug, e.g. if you are using an image that is
defined as `ghcr.io/somerepo/someimage`, Image Updater will infer the registry
from the `ghcr.io` prefix and use the Docker registry at `https://ghcr.io/v2`.

However, there may be reasons when you need to configure a custom registry, or
adapt the default settings to your requirements:

* Your registry uses a self-signed (or not publicly known) TLS certificate,
  and you want to either turn off TLS certificate verification or provide a
  custom CA certificate to be used for verification

* Your registry has its API endpoint at a different location. One notable
  example is Docker Hub, which uses `docker.io` as image prefix but the
  registry is at `registry-1.docker.io`. However, this quirk related to
  Docker Hub has been accounted for in Argo CD Image Updater, so you won't
  have to configure Docker Hub as a custom registry.

* You want to use a certain registry with deviations from the default settings,
  such as setting a custom rate limit, or configuring global credentials for
  accessing that registry.

* Your cluster's container engine is set up to use a different registry than
  `docker.io` as the default so that it does not pull images from Docker Hub
  when no prefix is specified, but e.g. from an on-premise registry.

## Can I use my registry without further configuration?

There is an easy way to find out by running the argocd-image-updater CLI in
[test mode](../install/testing.md). You can run the CLI either from your
workstation (if the registry is reachable from there) or from the pod deployed
to your cluster.

Assuming you have an image at `myregistry.com/somerepo/someimage`, and you
want to find out if Argo CD Image Updater could possibly connect to that
specific registry at `myregistry.com` and gather information about the image,
run the following command:

```
argocd-image-updater test myregistry.com/somerepo/someimage
```

You may also want to pass some options to that command to set the appropriate
[update strategy](../basics/update-strategies.md)
or
[credentials](../basics/authentication.md)
to access the registry.

The `argocd-image-updater test` command won't perform any modifications to
your cluster or your workloads.

## <a name="custom-registries"></a>Configuring custom registries
### <a name="registry-configuration-format"></a>Configuration format

Registries are configured in a configuration file in YAML syntax. That YAML
must have a top level key of `registries`, which holds a list of registries
to be configured. Each item in the list represents a single registry. You
can add as many registries as you like.

The following properties can be set to configure a registry. Most of these
properties are optional, unless otherwise stated:

  * `name` (mandatory) - The symbolic name for your registry. Can be any string,
    used only for informational purposes.

    Default value: _none_

    Example value: `My Custom Registry`

  * `prefix` - (mandatory) The lookup prefix for this registry. Must match a DNS
    name, and must be unique in the configuration.

    Default value: _none_

    Example value: `docker.io`

  * `api_url` - The URL to the API of the Docker registry. Must be a HTTP or
    HTTPS URL.

    Default value: _none_

    Example value: `https://registry-1.docker.io`

  * `defaultns`- Some registries (notably Docker Hub) have a default namespace,
    which can be specified here.

    Default value: _none_

    Example value: `library`

  * `default` - If set to true, use this registry as the default registry.
   
    Default value: `false`

    Example value: `true`
   
  * `credentials` - A string describing the credential source to use when
    accessing this registry and no other credentials are configured on the
    image level. For more information, refer to
    [Authenticating to container registries](../../basics/authentication/#auth-registries).

    Default value: _none_

    Example value: `pullsecret:argocd/dockerhub-secret`

  * `credsexpire` - Whether and when credentials should expire and be
    re-fetched. Must be specified as _Duration_ value, with an integer value
    and a unit suffix, i.e. `s` for seconds, `m` for minutes, or `h` for hours.

    Default value: _none (no expiry)_

    Example value: `5h`
  
  * `insecure` - Whether to turn off verification of the registry's TLS
    certificate. As the name implies, it's insecure and should not be used in
    production environments

    Default value: `false`

    Example value: `true`

  * `limit` - The rate limit (max. number of requests per second) to use for
    this registry, specified as integer.

    Default value: `20`

    Example value: `100`

The following is an example that configures two registries.

```yaml
registries:
- name: Docker Hub
  prefix: docker.io
  api_url: https://registry-1.docker.io
  credentials: secret:foo/bar#creds
  defaultns: library
  default: true
- name: RedHat Quay
  api_url: https://quay.io
  prefix: quay.io
  insecure: yes
  credentials: env:REGISTRY_SECRET
```

When running Argo CD Image Updater from the command line (e.g. using the `test`
command), the path to this YAML file can be specified with the command line
parameter `--registries-conf-path <path>`.

If you are running Argo CD Image Updater as a Kubernetes workload, you will
need to tie that up in the `argocd-image-updater-config` ConfigMap, as a multi
line string under the `registries.conf` key, e.g.:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-image-updater-config
data:
  registries.conf: |
    registries:
    - name: Docker Hub
      prefix: docker.io
      api_url: https://registry-1.docker.io
      credentials: secret:foo/bar#creds
      defaultns: library
      default: true
    - name: RedHat Quay
      api_url: https://quay.io
      prefix: quay.io
      insecure: yes
      credentials: env:REGISTRY_SECRET

```

!!!note
    Argo CD Image Updater pod must be restarted for changes to the registries
    configuration to take effect. There are plans to change this behaviour so
    that changes will be reload automatically in a future release.

### <a name="default-registry"></a>Configuring a default registry

!!!warning
    When you change the default registry for Argo CD Image Updater, make sure
    it matches the default registry set up in your cluster. If the default
    registries mismatch between the Argo CD Image Updater and your cluster,
    it is going to break your workloads.

Most container engines will follow Docker's approach and use Docker Hub as the
default registry when no registry is specified in the image's reference. For
example, in most environments, the two image references `argoproj/argocd` and
`docker.io/argoproj/argocd` are semantically the same, and would refer to the
same image stored in Docker Hub's registry. You can configure the default
registry if your cluster's container engine is configured to use a different
default registry than `docker.io`, which is the default for Argo CD Image
Updater in the standard configuration.

In order to set another registry to be used as default, you will have to set
the registry's `default` property to `true`, e.g. the following:

```yaml
registries:
- name: RedHat Quay
  api_url: https://quay.io
  prefix: quay.io
  default: true
```

would make `quay.io` the default registry, so `argoproj/argocd` would become
synonymous to `quay.io/argoproj/argocd`.

Please note that you can only configure one registry as the default registry.
If you set `default` to `true` for more than one registry's configuration,
Argo CD Image Updater will not load that configuration.

Also note, as stated previously, Docker Hub (i.e. `docker.io`) is the default
used by Image Updater without any additional configuration.

### <a name="default-namespace"></a>Configuring a registry's default namespace

Some registries provide a default namespace, that is, a namespace that doesn't
have to be specified in the image's slug. A good example for that is the
implicit `library` namespace on Docker Hub, that allows you to pull an image
from that library by just specifying the name of the image. Consider e.g. the
command `docker pull nginx`, which will pull from `docker.io/library/nginx`.

If your registry supports a default namespace and you would like this to be
used with Image Updater, you can configure this by setting the `defaultns`
property of the registry to the name of the namespace to use as the default.

The canonical example for Docker Hub would be:

```yaml
registries:
- name: Docker Hub
  api_url: https://registry-1.docker.io
  prefix: docker.io
  defaultns: library
```

### <a name="rate-limit"></a>Configuring a request rate limit for your registry

Some registries might be picky about how many requests per time they let you
perform from a client's connection. In order to play nicely, and to prevent
Image Updater from triggering rate limit failures on your registry, you can
set a rate limit for each of the registries you are using.

The rate limit can currently be configured as _requests per second_ allowed
and is specified via the `limit` configuration property. It takes an integer
as argument.

For example, the following connection would cap requests to `quay.io` to 10
per second:

```yaml
registries:
- name: RedHat Quay
  api_url: https://quay.io
  prefix: quay.io
  limit: 10
```

Please note that the limit considers all HTTP requests sent to the registry.
For inspecting a single tag of any given image, Image Updater may require
performing multiple requests to the registry.

The default for all registries is 20 requests per second. If your registry
doesn't impose limits upon you, raising this limit may significantly speed
up Argo CD Image Updater. But please be considerate and careful before you
increase the limit.

### <a name="registry-credentials"></a>Configuring default credentials for container registries

If you require authentication for accessing your registry, you can configure
a set of default credentials to use. If you have configured the credentials
on the registry level, it will be reused for any image processed from that
registry, unless an image configuration overrides it. A benefit of having
the credentials configured at the registry level is

Registry credentials are configured using the `credentials` property of a
registry's configuration.

### <a name="credentials-caching"></a>Credentials caching

By default, credentials specified in registry configuration are read once on
startup and then cached until `argocd-image-updater` is restarted. There are
two strategies to overcome this:

* Use per-image credentials in annotations - credentials will be read from
  their source every time an image update cycle is performed, and your
  credentials will always be up-to-date (i.e. if you update a secret).

* Specify credential expiry time in the registry configuration - if set, the
  registry credentials will have a defined lifetime, and will be re-read from
  the source after expiration. This can be especially useful if you generate
  credentials with a script which returns a token with a limited lifetime,
  i.e. for getting EKS credentials from the aws CLI. For example, if the
  token has a lifetime of 12 hours, you can set `credsexpire: 12h` and Argo
  CD Image Updater will get a new token after 12 hours.
