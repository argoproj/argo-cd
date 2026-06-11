# Mutual TLS (mTLS) for repo-server

> [!TIP]
> Looking for general TLS setup across Argo CD components? See [TLS configuration](./tls.md).

Argo CD supports mutual TLS (mTLS) between the `argocd-server` (API server), `argocd-application-controller`, `argocd-applicationset-controller`, and the `argocd-repo-server`. This ensures the repo-server only accepts connections from authorized clients that present a valid client certificate.

> [!NOTE]
> The repo-server runs a gRPC endpoint with TLS by default. Enabling mTLS adds client certificate verification on top of server-side TLS.

## Quickstart — default shared-cert setup

This is the simplest and recommended way to enable mTLS. A single client certificate and key are shared by all client components (`argocd-server`, `argocd-application-controller`, `argocd-applicationset-controller`, and `argocd-notifications-controller`). Argo CD auto-mounts the Secret into every relevant deployment — no manual volume or volumeMount configuration is required.

**Step 1 — Create the `argocd-repo-server-mtls` Secret** in the `argocd` namespace with the following keys:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: argocd-repo-server-mtls
  namespace: argocd
type: Opaque
data:
  # CA used by argocd-repo-server to verify incoming client certificates (enables mTLS)
  client-ca.crt: <BASE64_CA_PEM>
  # Shared client certificate and key presented by all client components
  client.crt: <BASE64_CLIENT_CERT_PEM>
  client.key: <BASE64_CLIENT_KEY_PEM>
  # Optional: CA used by clients to verify the repo-server's own TLS certificate
  # server-ca.crt: <BASE64_SERVER_CA_PEM>
```

The Secret is automatically mounted at `/app/config/reposerver/mtls` in every relevant pod. The repo-server's `--client-ca-path` flag defaults to `/app/config/reposerver/mtls/client-ca.crt`, 
so **mTLS is enabled automatically** as soon as the Secret exists — no additional repo-server configuration is required.

**Step 2 — Configure the client components** to present the shared cert by adding the following keys to `argocd-cmd-params-cm` (create the ConfigMap if it does not already exist):

```yaml
  # argocd-server
  server.repo.server.client.cert.path: "/app/config/reposerver/mtls/client.crt"
  server.repo.server.client.cert.key.path: "/app/config/reposerver/mtls/client.key"

  # argocd-application-controller
  controller.repo.server.client.cert.path: "/app/config/reposerver/mtls/client.crt"
  controller.repo.server.client.cert.key.path: "/app/config/reposerver/mtls/client.key"

  # argocd-applicationset-controller
  applicationsetcontroller.repo.server.client.cert.path: "/app/config/reposerver/mtls/client.crt"
  applicationsetcontroller.repo.server.client.cert.key.path: "/app/config/reposerver/mtls/client.key"

  # argocd-notifications-controller
  notificationscontroller.repo.server.client.cert.path: "/app/config/reposerver/mtls/client.crt"
  notificationscontroller.repo.server.client.cert.key.path: "/app/config/reposerver/mtls/client.key"
```

After applying these changes, restart the affected deployments (or wait for the rollout). The repo-server will automatically enable mTLS as soon as it reads the `client-ca.crt` from the mounted Secret — no further repo-server configuration is needed.

> [!NOTE]
> When mTLS is enabled, the repo-server automatically generates an ephemeral client certificate for its own internal health-check (liveness) self-connection. You don't need to change the readiness/liveness probes. On startup you should see a log line similar to:
>
> `Generated ephemeral health-check client certificate (CN=<value>)`

---

## Reference

The sections below document all available flags, ConfigMap keys, and advanced configuration options.

## Enabling mTLS on repo-server

To enable mTLS on the `argocd-repo-server`, provide a client CA certificate. When configured, the repo-server requires all clients to present a certificate signed by this CA.

### Command line flags

* `--client-ca-path`: Path to the client CA certificate file. Defaults to `/app/config/reposerver/mtls/client-ca.crt` (the auto-mounted Secret path). mTLS is enabled when the file exists; if the file is absent, mTLS is silently skipped.
* `--disable-tls`: Disables TLS for the repo-server gRPC endpoint. This is incompatible with `--client-ca-path`.

### Environment variables

* `ARGOCD_REPO_SERVER_CLIENT_CA_PATH`: Equivalent to `--client-ca-path`. Defaults to `/app/config/reposerver/mtls/client-ca.crt`.
* `ARGOCD_REPO_SERVER_DISABLE_TLS`: Equivalent to `--disable-tls`.

### Server certificate location (repo-server)

The repo-server's server certificate and key are read from:

* `/app/config/reposerver/tls/tls.crt`
* `/app/config/reposerver/tls/tls.key`

You can override the root config path with the `ARGOCD_APP_CONFIG_PATH` environment variable if you use a non-default layout.

## Configuring clients

The `argocd-server`, `argocd-application-controller`, and `argocd-applicationset-controller` must be configured with a client certificate and key to connect to a repo-server that has mTLS enabled. The certificate must be signed by the same CA configured on the repo-server.

> [!NOTE]
> **Legacy path vs. recommended path for TLS certificate validation**
>
> `--repo-server-strict-tls` (and `--argocd-repo-server-strict-tls` for the notifications controller)
> is the **legacy path**: when set, the component auto-discovers the repo-server certificate from
> the `argocd-repo-server-tls` Kubernetes secret. This flag is **deprecated** and may be removed
> in a future release.
>
> `--repo-server-ca-cert-path` (and `--argocd-repo-server-ca-cert-path` for the notifications controller)
> is the **recommended explicit path**: you provide the path to a CA certificate file directly.
> This is required for mTLS setups and gives you full control over which CA is trusted.
> Use `--repo-server-ca-cert-path` in all new deployments.

### argocd-server flags

* `--repo-server-client-cert-path`: Path to the client certificate file.
* `--repo-server-client-cert-key-path`: Path to the client certificate key file.
* `--repo-server-ca-cert-path`: Path to the CA certificate used to verify the repo-server's server certificate. This is the **recommended explicit path** for TLS validation and is required for mTLS setups.

### argocd-application-controller flags

* `--repo-server-client-cert-path`: Path to the client certificate file.
* `--repo-server-client-cert-key-path`: Path to the client certificate key file.
* `--repo-server-ca-cert-path`: Path to the CA certificate used to verify the repo-server's server certificate. This is the **recommended explicit path** for TLS validation and is required for mTLS setups.

### argocd-applicationset-controller flags

* `--repo-server-client-cert-path`: Path to the client certificate file.
* `--repo-server-client-cert-key-path`: Path to the client certificate key file.
* `--repo-server-ca-cert-path`: Path to the CA certificate used to verify the repo-server's server certificate. This is the **recommended explicit path** for TLS validation and is required for mTLS setups.

### argocd-cmd-params-cm ConfigMap keys

These environment variables can also be set via the `argocd-cmd-params-cm` ConfigMap, which is the recommended approach for Kubernetes deployments:

#### argocd-repo-server
* `reposerver.client.ca.path` — path to the client CA certificate file; defaults to `/app/config/reposerver/mtls/client-ca.crt`. The repo-server requires all gRPC clients to present a certificate signed by this CA when the file exists. mTLS is silently skipped if the file is absent.

#### argocd-server
* `server.repo.server.ca.cert.path` — path to the CA certificate for verifying the repo server's TLS certificate
* `server.repo.server.client.cert.path` — path to the client certificate for mTLS
* `server.repo.server.client.cert.key.path` — path to the client certificate key for mTLS

#### argocd-application-controller
* `controller.repo.server.ca.cert.path` — path to the CA certificate for verifying the repo server's TLS certificate
* `controller.repo.server.client.cert.path` — path to the client certificate for mTLS
* `controller.repo.server.client.cert.key.path` — path to the client certificate key for mTLS

#### argocd-applicationset-controller
* `applicationsetcontroller.repo.server.ca.cert.path` — path to the CA certificate for verifying the repo server's TLS certificate
* `applicationsetcontroller.repo.server.client.cert.path` — path to the client certificate for mTLS
* `applicationsetcontroller.repo.server.client.cert.key.path` — path to the client certificate key for mTLS

#### argocd-notifications-controller
* `notificationscontroller.repo.server.ca.cert.path` — path to the CA certificate for verifying the repo server's TLS certificate
* `notificationscontroller.repo.server.client.cert.path` — path to the client certificate for mTLS
* `notificationscontroller.repo.server.client.cert.key.path` — path to the client certificate key for mTLS

> [!IMPORTANT]
> Both `--repo-server-client-cert-path` and `--repo-server-client-cert-key-path` must be provided together. If you provide one without the other, the component will fail validation at startup.

## Shared vs. per-component client certificates

By default, the `argocd-repo-server-mtls` Secret uses a **single shared key/cert pair** (`client.crt` / `client.key`) that is mounted into every client component (`argocd-server`, `argocd-application-controller`, `argocd-applicationset-controller`, and `argocd-notifications-controller`). All components therefore present the same client certificate to the repo-server.

This is enough for most deployments. If you need the repo-server to distinguish which component is connecting (for example, to apply per-component authorization policies), you must issue separate certificates per component and configure each deployment to use its own cert.

> [!NOTE]
> The repo-server only verifies that the client certificate is signed by the configured client CA. It does not enforce per-component identity by default. Per-component certs are only meaningful if you add your own authorization logic on top of mTLS.

### Option A — Multiple keys in one Secret

Store all component certs in the single `argocd-repo-server-mtls` Secret under different key names, then customize the volume `items` projection in each deployment to expose only the relevant cert.

**1. Create the Secret with per-component keys:**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: argocd-repo-server-mtls
  namespace: argocd
type: Opaque
data:
  client-ca.crt: <BASE64_CA_PEM>
  # argocd-server client cert
  server-client.crt: <BASE64_SERVER_CLIENT_CERT_PEM>
  server-client.key: <BASE64_SERVER_CLIENT_KEY_PEM>
  # argocd-application-controller client cert
  controller-client.crt: <BASE64_CONTROLLER_CLIENT_CERT_PEM>
  controller-client.key: <BASE64_CONTROLLER_CLIENT_KEY_PEM>
  # argocd-applicationset-controller client cert
  appsetcontroller-client.crt: <BASE64_APPSET_CLIENT_CERT_PEM>
  appsetcontroller-client.key: <BASE64_APPSET_CLIENT_KEY_PEM>
  # argocd-notifications-controller client cert
  notifications-client.crt: <BASE64_NOTIFICATIONS_CLIENT_CERT_PEM>
  notifications-client.key: <BASE64_NOTIFICATIONS_CLIENT_KEY_PEM>
```

**2. Patch each deployment's volume to project only the relevant keys as `client.crt` / `client.key`:**

```yaml
# Example patch for argocd-server deployment
volumes:
- name: argocd-repo-server-mtls
  secret:
    secretName: argocd-repo-server-mtls
    items:
    - key: server-client.crt
      path: client.crt
    - key: server-client.key
      path: client.key
    - key: client-ca.crt
      path: client-ca.crt
```

Repeat with the appropriate key names for each component deployment. The mount path and ConfigMap keys remain unchanged — only the Secret `items` projection differs per deployment.

### Option B — One Secret per component

Create a separate Secret for each component. Each Secret follows the same format as `argocd-repo-server-mtls` but contains only that component's cert.

**1. Create per-component Secrets:**

```yaml
# argocd-server
apiVersion: v1
kind: Secret
metadata:
  name: argocd-repo-server-mtls-server
  namespace: argocd
type: Opaque
data:
  client.crt: <BASE64_SERVER_CLIENT_CERT_PEM>
  client.key: <BASE64_SERVER_CLIENT_KEY_PEM>
---
# argocd-application-controller
apiVersion: v1
kind: Secret
metadata:
  name: argocd-repo-server-mtls-controller
  namespace: argocd
type: Opaque
data:
  client.crt: <BASE64_CONTROLLER_CLIENT_CERT_PEM>
  client.key: <BASE64_CONTROLLER_CLIENT_KEY_PEM>
# ... repeat for argocd-applicationset-controller and argocd-notifications-controller
```

**2. Add a separate volume and volumeMount to each deployment, pointing to its own Secret:**

```yaml
# Example patch for argocd-server deployment
volumes:
- name: argocd-repo-server-mtls-server
  secret:
    secretName: argocd-repo-server-mtls-server
volumeMounts:
- name: argocd-repo-server-mtls-server
  mountPath: /app/config/reposerver/mtls
  readOnly: true
```

Repeat with the appropriate Secret name for each component deployment. The mount path and ConfigMap keys remain unchanged.

> [!IMPORTANT]
> With Option B, the default `argocd-repo-server-mtls` volume that Argo CD auto-mounts must be removed or overridden in each patched deployment, otherwise both volumes will compete for the same mount path.

## No per-component enforcement on the server side

Even if you configure separate client certificates per component (using Option A or Option B above), the repo-server **does not distinguish between them**. This is a fundamental characteristic of how the server-side TLS verification works, and operators should be aware of it before investing in per-component cert issuance.

The repo-server is configured with a single `--client-ca-path` flag, which points to one CA certificate file. Internally, the repository server loads that file into a single `x509.CertPool`. When a client connects, the TLS layer asks one question: **"Is this certificate signed by the configured CA?"** If yes, the connection is accepted. If no, it is rejected.

There is no binding between a specific component identity and a specific certificate. The server does not inspect the certificate's Subject, SAN, or any other field to determine *which* component is connecting. Any client that holds a certificate signed by the trusted CA will pass — regardless of whether it is `argocd-server`, `argocd-application-controller`, or any other process with access to a valid cert.

All client certificates must be signed by a CA trusted by the repo-server. The `--client-ca-path` flag accepts a PEM file containing one or more CA certificates. 
If you issue separate certs per component from different CAs, concatenate those CAs into a single bundle file and pass that bundle to `--client-ca-path`. 
The server will trust any cert signed by any CA in the bundle, but will still not distinguish which component is connecting.

> [!NOTE]
> Per-component certificates are only meaningful if you implement your own authorization logic on top of mTLS — for example, an Envoy sidecar or a custom gRPC interceptor that inspects the peer certificate's Subject/SAN and enforces component-level access policies. Out of the box, Argo CD mTLS provides **authentication** (only cert-holding clients can connect) but not **authorization** (any authenticated client can call any RPC).

## Verifying mTLS is active

After deploying:

1. Check repo-server logs for the health-check certificate message when `--client-ca-path` is set:

   `Generated ephemeral health-check client certificate (CN=...)`

2. Attempt a connection from a pod without the client certificate — it should fail with a TLS error due to missing client authentication.
3. Connections from `argocd-server`/`argocd-application-controller`/`argocd-applicationset-controller` should succeed when configured with matching client cert/key.

## Troubleshooting

- Error: `--client-ca-path cannot be used when --disable-tls is enabled`
  - Remove `--disable-tls` (or unset `ARGOCD_REPO_SERVER_DISABLE_TLS`) when enabling mTLS.
- One of `--repo-server-client-cert-path` / `--repo-server-client-cert-key-path` missing
  - Provide both flags (or the corresponding environment variables) together.
- Custom CA for repo-server server certificate
  - Provide `--repo-server-ca-cert-path` on clients so they can verify the repo-server's server certificate.
