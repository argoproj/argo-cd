# TLS configuration

Argo CD provides two inbound TLS endpoints that can be configured:

* The user-facing endpoint of the `argocd-server` workload which serves the UI
  and the API
* The endpoint of the `argocd-repo-server`, which is accessed by `argocd-server`
  and `argocd-application-controller` workloads to request repository
  operations.

By default, and without further configuration, both of these endpoints will be
set-up to use an automatically generated, self-signed certificate. However,
most users will want to explicitly configure the certificates for these TLS
endpoints, possibly using automated means such as `cert-manager` or using
their own dedicated Certificate Authority.

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
  method is considered deprecated, and only exists for purposes of backwards
  compatiblity. Changing `argocd-secret` should not be used to override the
  TLS certificate anymore.

Argo CD decides which TLS certificate to use for the endpoint of
`argocd-server` as follows:

* If the `argocd-server-tls` secret exists and contains a valid key pair in the
  `tls.crt` and `tls.key` keys, this will be used for the certificate of the
  endpoint of `argocd-server`.
* Otherwise, if the `argocd-secret` secret contains a valid key pair in the 
 `tls.crt` and `tls.key` keys, this will be used as certificate for the
  endpoint of `argocd-server`.
* If no `tls.crt` and `tls.key` keys are found in neither of the two mentioned
  secrets, Argo CD will generate a self-signed certificate and persist it in
  the `argocd-secret` secret.

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
and will not require restart of the pods to use a renewed certificate.

## Configuring inbound TLS for argocd-repo-server

### Inbound TLS options for argocd-repo-server

You can configure certain TLS options for the `argocd-repo-server` workload by
setting command line parameters. The following parameters are available:

|Parameter|Default|Description|
|---------|-------|-----------|
|`--disable-tls`|`false`|Disables TLS completely|
|`--tlsminversion`|`1.2`|The minimum TLS version to be offered to clients|
|`--tlsmaxversion`|`1.3`|The maximum TLS version to be offered to clients|
|`--tlsciphers`|`TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:TLS_RSA_WITH_AES_256_GCM_SHA384`|A colon separated list of TLS cipher suites to be offered to clients|

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

Please note, that as opposed to `argocd-server`, the `argocd-repo-server` is
not able to pick up changes to this secret automatically. If you create (or
update) this secret, the `argocd-repo-server` pods need to be restarted.

Also note, that the certificate should be issued with the correct SAN entries
for the `argocd-repo-server`, containing at least the entries for
`DNS:argocd-repo-server` and `DNS:argocd-repo-server.argo-cd.svc` depending
on how your workloads connect to the repository server.

## Configuring TLS between Argo CD components

### Configuring TLS to argocd-repo-server

Both `argocd-server` and `argocd-application-controller` communicate with the
`argocd-repo-server` using a gRPC API over TLS. By default,
`argocd-repo-server` generates a non-persistant, self signed certificate
to use for its gRPC endpoint on startup. Because the `argocd-repo-server` has
no means to connect to the K8s control plane API, this certificate is not
being available to outside consumers for verification. Both, the
`argocd-server` and `argocd-application-server` will use a non-validating
connection to the `argocd-repo-server` for this reason.

To change this behavior to be more secure by having the `argocd-server` and
`argocd-application-controller` validate the TLS certificate of the
`argocd-repo-server` endpoint, the following steps need to be performed:

* Create a persistent TLS certificate to be used by `argocd-repo-server`, as
  shown above
* Restart the `argocd-repo-server` pod(s)
* Modify the pod startup parameters for `argocd-server` and
  `argocd-application-controller` to include the `--repo-server-strict-tls`
  parameter.

The `argocd-server` and `argocd-application-controller` workloads will now
validate the TLS certificate of the `argocd-repo-server` by using the
certificate stored in the `argocd-repo-server-tls` secret.

!!!note "Certificate expiry"
    Please make sure that the certificate has a proper life time. Keep in
    mind that when you have to replace the certificate, all workloads have
    to be restarted in order to properly work again.

### Disabling TLS to argocd-repo-server

In some scenarios where mTLS through side-car proxies is involved (e.g.
in a service mesh), you may want configure the connections between the
`argocd-server` and `argocd-application-controller` to `argocd-repo-server`
to not use TLS at all.

In this case, you will need to:

* Configure `argocd-repo-server` with TLS on the gRPC API disabled by specifying
  the `--disable-tls` parameter to the pod container's startup arguments
* Configure `argocd-server` and `argocd-application-controller` to not use TLS
  for connections to the `argocd-repo-server` by specifying the parameter
  `--repo-server-plaintext` to the pod container's startup arguments
* Configure `argocd-server` and `argocd-application-controller` to connect to
  the side-car instead of directly to the `argocd-repo-server` service by
  specifying its address via the `--repo-server <address>` parameter

After this change, the `argocd-server` and `argocd-application-controller` will
use a plain text connection to the side-car proxy, that will handle all aspects
of TLS to the `argocd-repo-server`'s TLS side-car proxy.
