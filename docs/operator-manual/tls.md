# TLS configuration

> [!TIP]
> Need repo-server mutual TLS between components? See [Mutual TLS (mTLS) for repo-server](./mtls.md).

Argo CD provides three inbound TLS endpoints that can be configured:

* The user-facing endpoint of the `argocd-server` workload, which serves the UI
  and the API
* The endpoint of the `argocd-repo-server`, which is accessed by `argocd-server`
  and `argocd-application-controller` workloads to request repository
  operations.
* The endpoint of the `argocd-dex-server`, which is accessed by `argocd-server`
  to handle OIDC authentication.

By default, and without further configuration, these endpoints will be
set up to use an automatically generated, self-signed certificate. However,
most users will want to explicitly configure the certificates for these TLS
endpoints, possibly using automated means such as `cert-manager` or using
their own dedicated Certificate Authority.

## TLS Configuration Quick Reference

### Certificate Configuration Overview

| Component | Secret Name | Hot Reload | Default Cert | Required SAN Entries |
|-----------|-------------|------------|---------------|---------------------|
| `argocd-server` | `argocd-server-tls` | ✅ Yes | Self-signed | External hostname (e.g., `argocd.example.com`) |
| `argocd-repo-server` | `argocd-repo-server-tls` | ❌ Restart required | Self-signed | `DNS:argocd-repo-server`, `DNS:argocd-repo-server.argocd.svc` |
| `argocd-dex-server` | `argocd-dex-server-tls` | ❌ Restart required | Self-signed | `DNS:argocd-dex-server`, `DNS:argocd-dex-server.argocd.svc` |

### Inter-Component TLS

| Connection | Recommended Parameter | Legacy Parameter (deprecated) | Plain Text Parameter | Default Behavior |
|------------|----------------------|-------------------------------|---------------------|------------------|
| `argocd-server` → `argocd-repo-server` | `--repo-server-ca-cert-path` | `--repo-server-strict-tls` | `--repo-server-plaintext` | Non-validating TLS |
| `argocd-server` → `argocd-dex-server` | — | `--dex-server-strict-tls` | `--dex-server-plaintext` | Non-validating TLS |
| `argocd-application-controller` → `argocd-repo-server` | `--repo-server-ca-cert-path` | `--repo-server-strict-tls` | `--repo-server-plaintext` | Non-validating TLS |
| `argocd-applicationset-controller` → `argocd-repo-server` | `--repo-server-ca-cert-path` | `--repo-server-strict-tls` | `--repo-server-plaintext` | Non-validating TLS |
| `argocd-notifications-controller` → `argocd-repo-server` | `--argocd-repo-server-ca-cert-path` | `--argocd-repo-server-strict-tls` | `--argocd-repo-server-plaintext` | Non-validating TLS |

### Certificate Priority (argocd-server only)

1. `argocd-server-tls` secret (recommended)
2. `argocd-secret` secret (deprecated)
3. Auto-generated self-signed certificate (only when `argocd-server-tls` does not exist, or exists and is annotated with `argocd.argoproj.io/self-signed: "true"`)

## Configuring TLS for argocd-server

### Inbound TLS options for argocd-server

You can configure certain TLS options for the `argocd-server` workload by
setting command line parameters. The following parameters are available:

|Parameter|Default|Description|
|---------|-------|-----------|
|`--insecure`|`false`|Disables TLS completely|
|`--tlsminversion`|`1.2`|The minimum TLS version to be offered to clients|
|`--tlsmaxversion`|`1.3`|The maximum TLS version to be offered to clients|
|`--tlsciphers`|`TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:TLS_RSA_WITH_AES_256_GCM_SHA384`|A colon separated list of TLS cipher suites to be offered to clients|

### TLS certificates used by argocd-server

There are two ways to configure the TLS certificates used by `argocd-server`:

* Setting the `tls.crt` and `tls.key` keys in the `argocd-server-tls` secret
  to hold PEM data of the certificate and the corresponding private key. The
  `argocd-server-tls` secret may be of type `tls`, but does not have to be.
* Setting the `tls.crt` and `tls.key` keys in the `argocd-secret` secret to
  hold PEM data of the certificate and the corresponding private key. This
  method is considered deprecated and only exists for purposes of backwards
  compatibility. Changing `argocd-secret` should not be used to override the
  TLS certificate anymore.

Argo CD decides which TLS certificate to use for the endpoint of
`argocd-server` as follows:

* If the `argocd-server-tls` secret exists and contains a valid key pair in the
  `tls.crt` and `tls.key` keys, this will be used for the certificate of the
  endpoint of `argocd-server`.
* Otherwise, if the `argocd-secret` secret contains a valid key pair in the
  `tls.crt` and `tls.key` keys, this will be used as the certificate for the
  endpoint of `argocd-server`. This fallback is deprecated — see warning below.
* If the `argocd-server-tls` secret does not exist, or exists and is annotated
  with `argocd.argoproj.io/self-signed: "true"` but contains no valid key pair,
  Argo CD will generate a self-signed certificate, persist it in the
  `argocd-server-tls` secret, and stamp it with that annotation. Argo CD checks
  this certificate on every `argocd-server` startup and regenerates it if it has
  expired.
* If the `argocd-server-tls` secret exists **without** the
  `argocd.argoproj.io/self-signed: "true"` annotation and no valid key pair can
  be loaded from it, Argo CD will not overwrite the secret. This protects
  operator-managed secrets (e.g. those managed by `cert-manager`) from being
  silently replaced during transient states such as the secret not having been
  populated yet. Argo CD logs a warning and serves an in-memory self-signed
  certificate instead, which is not persisted and is regenerated on every
  restart. Once the secret is populated with a valid key pair, Argo CD picks it
  up and uses it.

> [!WARNING]
> Storing TLS certificates in `argocd-secret` is deprecated since Argo CD v2.1
> and will be removed in a future major version. Please move `tls.crt` and
> `tls.key` to the `argocd-server-tls` secret. Argo CD will log a warning on
> startup if it detects TLS data in `argocd-secret`.

The `argocd-server-tls` secret contains only information for TLS configuration
to be used by `argocd-server` and is safe to be managed via third-party tools
such as `cert-manager` or `SealedSecrets`

To create this secret manually from an existing key pair, you can use `kubectl`:

```shell
kubectl create -n argocd secret tls argocd-server-tls \
  --cert=/path/to/cert.pem \
  --key=/path/to/key.pem
```

Argo CD will pick up changes to the `argocd-server-tls` secret automatically
and will not require restarting to use a renewed certificate.

### Certificate ownership and the `self-signed` annotation

Argo CD uses the `argocd.argoproj.io/self-signed: "true"` annotation on the
`argocd-server-tls` secret to record that it generated the certificate and is
therefore allowed to replace it. The annotation is what separates a certificate
Argo CD owns from one you own:

| Annotation | Who owns the certificate | What Argo CD does |
|------------|--------------------------|-------------------|
| Present | Argo CD | Serves the certificate, and regenerates it on startup once it has expired |
| Absent | You (or a tool such as `cert-manager`) | Serves the certificate, and never writes to the secret |

> [!WARNING]
> Do not add or remove this annotation by hand, and make sure your GitOps tooling
> does not strip it. Because the annotation controls ownership, changing it puts
> the secret into a state that is easy to misread:
>
> * **Removing it from an Argo CD-generated certificate** makes Argo CD treat that
>   certificate as yours. Argo CD will keep serving it but will no longer renew it,
>   so once it expires clients get an expired certificate and nothing recovers it
>   automatically. If the secret is also emptied, Argo CD falls back to an in-memory
>   certificate that changes on every restart.
> * **Adding it to a certificate you manage** hands ownership to Argo CD, which will
>   overwrite your certificate with a self-signed one the next time it regenerates.
>
> This matters in particular when `argocd-server-tls` is templated by Helm, Kustomize,
> or an external secrets operator: a template that renders a fixed annotation set will
> silently drop the annotation on every reconcile.

To recover a secret whose annotation was removed, either restore the annotation to
let Argo CD manage the certificate again, or delete the secret and let Argo CD
recreate it:

```shell
kubectl annotate -n argocd secret argocd-server-tls argocd.argoproj.io/self-signed="true" --overwrite
```

### Migrating TLS certificates from `argocd-secret`

Argo CD stores the auto-generated certificate in `argocd-server-tls`. Older
installations may still hold `tls.crt` and `tls.key` in `argocd-secret`, which is
deprecated. If they are present, `argocd-server` logs this on startup:

```
Storing TLS certificates in argocd-secret is deprecated and will be removed in a
future major version. Please move tls.crt and tls.key to the argocd-server-tls
secret and remove them from argocd-secret.
```

The warning is logged on every startup until the keys are removed from
`argocd-secret`. Argo CD keeps reading them in the meantime, so nothing breaks
while you migrate.

First, find out whose certificate is in `argocd-secret`:

```shell
kubectl get secret argocd-secret -n argocd -o jsonpath='{.data.tls\.crt}' \
  | base64 -d | openssl x509 -noout -subject -issuer -dates
```

A certificate that Argo CD generated itself is self-signed with
`O = Argo CD`. Anything else is a certificate you or your tooling put there.

On the first startup after upgrading, Argo CD copies the key pair from
`argocd-secret` into `argocd-server-tls` for you and annotates it with
`argocd.argoproj.io/self-signed: "true"`, so the certificate being served does not
change. What you do next depends on whose certificate it is.

**If the certificate was generated by Argo CD**, there is nothing left to move. Skip
to removing the deprecated keys below.

**If the certificate is your own**, remove the annotation from the copy, otherwise
Argo CD considers itself the owner and will replace your certificate with a
self-signed one once it expires:

```shell
kubectl annotate -n argocd secret argocd-server-tls argocd.argoproj.io/self-signed-
```

If you would rather manage the secret from scratch — for example to have
`cert-manager` or your GitOps tooling own it — delete the copy Argo CD made and
recreate it from your key pair:

```shell
kubectl delete -n argocd secret argocd-server-tls
kubectl create -n argocd secret tls argocd-server-tls \
  --cert=/path/to/cert.pem \
  --key=/path/to/key.pem
```

Finally, remove the deprecated keys from `argocd-secret`:

```shell
kubectl patch secret argocd-secret -n argocd \
  --type=json \
  -p='[{"op":"remove","path":"/data/tls.crt"},{"op":"remove","path":"/data/tls.key"}]'
```

> [!NOTE]
> Create `argocd-server-tls` before removing the keys from `argocd-secret`. If both
> locations are empty, Argo CD generates a new self-signed certificate, and clients
> that pinned or trusted the previous one will need to trust the new one.

## Configuring inbound TLS for argocd-repo-server

### Inbound TLS options for argocd-repo-server

You can configure certain TLS options for the `argocd-repo-server` workload by
setting command line parameters. The following parameters are available:

|Parameter|Default|Description|
|---------|-------|-----------|
|`--disable-tls`|`false`|Disables TLS completely|
|`--tlsminversion`|`1.2`|The minimum TLS version to be offered to clients|
|`--tlsmaxversion`|`1.3`|The maximum TLS version to be offered to clients|
|`--tlsciphers`|`TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:TLS_RSA_WITH_AES_256_GCM_SHA384`|A colon-separated list of TLS cipher suites to be offered to clients|

### Inbound TLS certificates used by argocd-repo-server

To configure the TLS certificate used by the `argocd-repo-server` workload,
create a secret named `argocd-repo-server-tls` in the namespace where Argo CD
is running in with the certificate's key pair stored in `tls.crt` and
`tls.key` keys. If this secret does not exist, `argocd-repo-server` will
generate and use a self-signed certificate.

To create this secret, you can use `kubectl`:

```shell
kubectl create -n argocd secret tls argocd-repo-server-tls \
  --cert=/path/to/cert.pem \
  --key=/path/to/key.pem
```

If the certificate is self-signed, you will also need to add `ca.crt` to the secret
with the contents of your CA certificate.

Please note, that as opposed to `argocd-server`, the `argocd-repo-server` is
not able to pick up changes to this secret automatically. If you create (or
update) this secret, the `argocd-repo-server` pods need to be restarted.

Also note, that the certificate should be issued with the correct SAN entries
for the `argocd-repo-server`, containing at least the entries for
`DNS:argocd-repo-server` and `DNS:argocd-repo-server.argo-cd.svc` depending
on how your workloads connect to the repository server.

## Configuring inbound TLS for argocd-dex-server

### Inbound TLS options for argocd-dex-server

You can configure certain TLS options for the `argocd-dex-server` workload by
setting command line parameters. The following parameters are available:

|Parameter|Default|Description|
|---------|-------|-----------|
|`--disable-tls`|`false`|Disables TLS completely|

### Inbound TLS certificates used by argocd-dex-server

To configure the TLS certificate used by the `argocd-dex-server` workload,
create a secret named `argocd-dex-server-tls` in the namespace where Argo CD
is running in with the certificate's key pair stored in `tls.crt` and
`tls.key` keys. If this secret does not exist, `argocd-dex-server` will
generate and use a self-signed certificate.

To create this secret, you can use `kubectl`:

```shell
kubectl create -n argocd secret tls argocd-dex-server-tls \
  --cert=/path/to/cert.pem \
  --key=/path/to/key.pem
```

If the certificate is self-signed, you will also need to add `ca.crt` to the secret
with the contents of your CA certificate.

Please note, that as opposed to `argocd-server`, the `argocd-dex-server` is
not able to pick up changes to this secret automatically. If you create (or
update) this secret, the `argocd-dex-server` pods need to be restarted.

Also note, that the certificate should be issued with the correct SAN entries
for the `argocd-dex-server`, containing at least the entries for
`DNS:argocd-dex-server` and `DNS:argocd-dex-server.argo-cd.svc` depending
on how your workloads connect to the repository server.

## Configuring TLS between Argo CD components

### Configuring TLS to argocd-repo-server

The components `argocd-server`, `argocd-application-controller`, `argocd-notifications-controller`, 
and `argocd-applicationset-controller` communicate with the `argocd-repo-server` 
using a gRPC API over TLS. By default, `argocd-repo-server` generates a non-persistent, 
self-signed certificate to use for its gRPC endpoint on startup. Because the 
`argocd-repo-server` has no means to connect to the K8s control plane API, this certificate 
is not available to outside consumers for verification. These components will use a 
non-validating connection to the `argocd-repo-server` for this reason.

To change this behavior to be more secure by having these components validate the TLS certificate of the
`argocd-repo-server` endpoint, the following steps need to be performed:

* Create a persistent TLS certificate to be used by `argocd-repo-server`, as
  shown above
* Restart the `argocd-repo-server` pod(s)
* Modify the pod startup parameters for `argocd-server`, `argocd-application-controller`,
  and `argocd-applicationset-controller` to include the
  `--repo-server-ca-cert-path` parameter pointing to the CA certificate file.
* Modify the pod startup parameters for `argocd-notifications-controller` to include the
  `--argocd-repo-server-ca-cert-path` parameter pointing to the CA certificate file.

The `argocd-server`, `argocd-application-controller`, `argocd-notifications-controller`,
and `argocd-applicationset-controller` workloads will now
validate the TLS certificate of the `argocd-repo-server` using the provided CA certificate.

> [!NOTE]
> **Legacy path vs. recommended path**
>
> `--repo-server-strict-tls` (and `--argocd-repo-server-strict-tls` for the notifications controller)
> is the **legacy path**: when set, the component auto-discovers the repo-server certificate from
> the `argocd-repo-server-tls` Kubernetes secret. This flag is **deprecated** and may be removed
> in a future release.
>
> `--repo-server-ca-cert-path` (and `--argocd-repo-server-ca-cert-path` for the notifications controller)
> is the **recommended explicit path**: you provide the path to a CA certificate file directly.
> This is required for mTLS setups and gives you full control over which CA is trusted.
> See [Mutual TLS (mTLS) for repo-server](./mtls.md) for details.

> [!NOTE]
> **Certificate expiry**
>
> Please make sure that the certificate has a proper lifetime. Remember, 
> when replacing certificates, all workloads must be restarted to pick up
> the certificate and work properly.

### Configuring TLS to argocd-dex-server

`argocd-server` communicates with the `argocd-dex-server` using an HTTPS API
over TLS. By default, `argocd-dex-server` generates a non-persistent, self-signed 
certificate for its HTTPS endpoint on startup. Because `argocd-dex-server` 
has no means to connect to the K8s control plane API, this certificate is not 
available to outside consumers for verification. `argocd-server` will use a 
non-validating connection to `argocd-dex-server` for this reason.

To change this behavior to be more secure by having the `argocd-server` validate 
the TLS certificate of the `argocd-dex-server` endpoint, the following steps need
to be performed:

* Create a persistent TLS certificate to be used by `argocd-dex-server`, as
  shown above
* Restart the `argocd-dex-server` pod(s)
* Modify the pod startup parameters for `argocd-server` to include the 
`--dex-server-strict-tls` parameter.

The `argocd-server` workload will now validate the TLS certificate of the
`argocd-dex-server` by using the certificate stored in the `argocd-dex-server-tls`
secret.

> [!NOTE]
> **Certificate expiry**
>
> Please make sure that the certificate has a proper lifetime. Remember, 
> when replacing certificates, all workloads must be restarted to pick up
> the certificate and work properly.


To configure TLS version for the bundled Dex server, update the `argocd-cm` ConfigMap:

```yaml
  apiVersion: v1
  kind: ConfigMap
  metadata:
    name: argocd-cm
  data:
    dex.config: |
      web:
        tlsMinVersion: "1.2"
```

### Disabling TLS to argocd-repo-server

In some scenarios where mTLS through sidecar proxies is involved (e.g.
in a service mesh), you may want to configure the connections between the
`argocd-server`, `argocd-application-controller`, `argocd-notifications-controller`, 
and `argocd-applicationset-controller` to `argocd-repo-server`
to not use TLS at all.

In this case, you will need to:

* Configure `argocd-repo-server` with TLS on the gRPC API disabled by specifying
  the `--disable-tls` parameter to the pod container's startup arguments.
  Also, consider restricting listening addresses to the loopback interface by specifying
  `--listen 127.0.0.1` parameter, so that the insecure endpoint is not exposed on
  the pod's network interfaces, but still available to the sidecar container.
* Configure `argocd-server`, `argocd-application-controller`, 
  and `argocd-applicationset-controller` to not use TLS
  for connections to the `argocd-repo-server` by specifying the parameter
  `--repo-server-plaintext` to the pod container's startup arguments
* Modify the pod startup parameters for `argocd-notifications-controller` to include the
  `--argocd-repo-server-plaintext` parameter
* Configure `argocd-server` and `argocd-application-controller` to connect to
  the sidecar instead of directly to the `argocd-repo-server` service by
  specifying its address via the `--repo-server <address>` parameter

After this change, `argocd-server`, `argocd-application-controller`, `argocd-notifications-controller`, 
and `argocd-applicationset-controller` will
use a plain text connection to the sidecar proxy, which will handle all aspects
of TLS to `argocd-repo-server`'s TLS sidecar proxy.

### Disabling TLS to argocd-dex-server

In some scenarios where mTLS through sidecar proxies is involved (e.g.
in a service mesh), you may want to configure the connections between
`argocd-server` to `argocd-dex-server` to not use TLS at all.

In this case, you will need to:

* Configure `argocd-dex-server` with TLS on the HTTPS API disabled by specifying
  the `--disable-tls` parameter to the pod container's startup arguments
* Configure `argocd-server` to not use TLS for connections to `argocd-dex-server` 
  by specifying the parameter `--dex-server-plaintext` to the pod container's startup
  arguments
* Configure `argocd-server` to connect to the sidecar instead of directly to the 
  `argocd-dex-server` service by specifying its address via the `--dex-server <address>`
  parameter

After this change, `argocd-server` will use a plain text connection to the sidecar 
proxy, that will handle all aspects of TLS to the `argocd-dex-server`'s TLS sidecar proxy.

