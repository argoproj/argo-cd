# Mutual TLS (mTLS) for repo-server

> [!TIP]
> Looking for general TLS setup across Argo CD components? See [TLS configuration](./tls.md).

Argo CD supports mutual TLS (mTLS) between the `argocd-server` (API server), `argocd-application-controller`, `argocd-applicationset-controller`, and the `argocd-repo-server`. This ensures the repo-server only accepts connections from authorized clients that present a valid client certificate.

> [!NOTE]
> The repo-server runs a gRPC endpoint with TLS by default. Enabling mTLS adds client certificate verification on top of server-side TLS.

## Enabling mTLS on repo-server

To enable mTLS on the `argocd-repo-server`, provide a client CA certificate. When configured, the repo-server requires all clients to present a certificate signed by this CA.

### Command line flags

* `--client-ca-path`: Path to the client CA certificate file. When set, repo-server requires client certificates signed by this CA.
* `--disable-tls`: Disables TLS for the repo-server gRPC endpoint. This is incompatible with `--client-ca-path`.

### Environment variables

* `ARGOCD_REPO_SERVER_CLIENT_CA_PATH`: Equivalent to `--client-ca-path`.
* `ARGOCD_REPO_SERVER_DISABLE_TLS`: Equivalent to `--disable-tls`.

> [!NOTE]
> When mTLS is enabled, the repo-server automatically generates an ephemeral client certificate for its own internal health-check (liveness) self-connection and adds it to the in-memory client CA pool. You don't need to change the readiness/liveness probes. On startup you should see a log line similar to:
>
> `Generated ephemeral health-check client certificate (CN=<value>)`

### Server certificate location (repo-server)

The repo-server's server certificate and key are read from:

* `/app/config/reposerver/tls/tls.crt`
* `/app/config/reposerver/tls/tls.key`

You can override the root config path with the `ARGOCD_APP_CONFIG_PATH` environment variable if you use a non-default layout.

## Configuring clients

The `argocd-server`, `argocd-application-controller`, and `argocd-applicationset-controller` must be configured with a client certificate and key to connect to a repo-server that has mTLS enabled. The certificate must be signed by the same CA configured on the repo-server.

### argocd-server flags

* `--repo-server-client-cert`: Path to the client certificate file.
* `--repo-server-client-cert-key`: Path to the client certificate key file.
* `--repo-server-ca-cert`: Path to the CA certificate used to verify the repo-server's server certificate (optional, when using a custom CA).

### argocd-application-controller flags

* `--repo-server-client-cert`: Path to the client certificate file.
* `--repo-server-client-cert-key`: Path to the client certificate key file.
* `--repo-server-ca-cert`: Path to the CA certificate used to verify the repo-server's server certificate (optional, when using a custom CA).

### argocd-applicationset-controller flags

* `--repo-server-client-cert`: Path to the client certificate file.
* `--repo-server-client-cert-key`: Path to the client certificate key file.
* `--repo-server-ca-cert`: Path to the CA certificate used to verify the repo-server's server certificate (optional, when using a custom CA).

### Environment variables

The environment variables follow the component's prefix and mirror the flags:

#### argocd-server
* `ARGOCD_SERVER_REPO_SERVER_CLIENT_CERT_PATH`
* `ARGOCD_SERVER_REPO_SERVER_CLIENT_CERT_KEY_PATH`
* `ARGOCD_SERVER_REPO_SERVER_CA_CERT_PATH`

#### argocd-application-controller
* `ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER_CLIENT_CERT_PATH`
* `ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER_CLIENT_CERT_KEY_PATH`
* `ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER_CA_CERT_PATH`

#### argocd-applicationset-controller
* `ARGOCD_APPLICATIONSET_CONTROLLER_REPO_SERVER_CLIENT_CERT_PATH`
* `ARGOCD_APPLICATIONSET_CONTROLLER_REPO_SERVER_CLIENT_CERT_KEY_PATH`
* `ARGOCD_APPLICATIONSET_CONTROLLER_REPO_SERVER_CA_CERT_PATH`

#### argocd-notifications-controller
* `ARGOCD_NOTIFICATION_CONTROLLER_REPO_SERVER_CLIENT_CERT_PATH`
* `ARGOCD_NOTIFICATION_CONTROLLER_REPO_SERVER_CLIENT_CERT_KEY_PATH`
* `ARGOCD_NOTIFICATION_CONTROLLER_REPO_SERVER_CA_CERT_PATH`

### argocd-cmd-params-cm ConfigMap keys

These environment variables can also be set via the `argocd-cmd-params-cm` ConfigMap, which is the recommended approach for Kubernetes deployments:

#### argocd-server
* `server.repo.server.ca.cert` — path to the CA certificate for verifying the repo server's TLS certificate
* `server.repo.server.client.cert` — path to the client certificate for mTLS
* `server.repo.server.client.cert.key` — path to the client certificate key for mTLS

#### argocd-application-controller
* `controller.repo.server.ca.cert` — path to the CA certificate for verifying the repo server's TLS certificate
* `controller.repo.server.client.cert` — path to the client certificate for mTLS
* `controller.repo.server.client.cert.key` — path to the client certificate key for mTLS

#### argocd-applicationset-controller
* `applicationsetcontroller.repo.server.ca.cert` — path to the CA certificate for verifying the repo server's TLS certificate
* `applicationsetcontroller.repo.server.client.cert` — path to the client certificate for mTLS
* `applicationsetcontroller.repo.server.client.cert.key` — path to the client certificate key for mTLS

#### argocd-notifications-controller
* `notificationscontroller.repo.server.ca.cert` — path to the CA certificate for verifying the repo server's TLS certificate
* `notificationscontroller.repo.server.client.cert` — path to the client certificate for mTLS
* `notificationscontroller.repo.server.client.cert.key` — path to the client certificate key for mTLS

> [!IMPORTANT]
> Both `--repo-server-client-cert` and `--repo-server-client-cert-key` must be provided together. If you provide one without the other, the component will fail validation at startup.

## Deployment using Kubernetes Secrets

Argo CD automatically mounts the optional `argocd-repo-server-mtls` Kubernetes Secret into all relevant deployments, following the same pattern as `argocd-repo-server-tls`. You only need to create the secret with the correct keys — no manual volume or volumeMount configuration is required.

The secret is mounted at:
- `/app/config/reposerver/mtls` in `argocd-repo-server`, `argocd-server`, `argocd-application-controller`, `argocd-applicationset-controller`, and `argocd-notifications-controller`.

### Secret format

Create the secret with the following keys:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: argocd-repo-server-mtls
  namespace: argocd
type: Opaque
data:
  # Required for argocd-repo-server (server-side mTLS): CA used to verify client certs
  client-ca.crt: <BASE64_CA_PEM>
  # Required for clients (argocd-server, argocd-application-controller, etc.)
  client.crt: <BASE64_CLIENT_CERT_PEM>
  client.key: <BASE64_CLIENT_KEY_PEM>
  # Optional: CA used by clients to verify the repo-server's server certificate
  # ca.crt: <BASE64_SERVER_CA_PEM>
```

### Configuring argocd-repo-server (enable mTLS)

Point `--client-ca-path` to the automatically mounted file:

```yaml
args:
- --client-ca-path
- /app/config/reposerver/mtls/client-ca.crt
```

### Configuring clients (argocd-server, argocd-application-controller, argocd-applicationset-controller, argocd-notifications-controller)

The recommended approach is to set the cert paths via `argocd-cmd-params-cm`. The deployments already expose the corresponding env vars from the ConfigMap, so you only need to patch the ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cmd-params-cm
  namespace: argocd
data:
  # For argocd-server:
  server.repo.server.client.cert: /app/config/reposerver/mtls/client.crt
  server.repo.server.client.cert.key: /app/config/reposerver/mtls/client.key
  # server.repo.server.ca.cert: /app/config/reposerver/mtls/ca.crt  # optional

  # For argocd-application-controller:
  controller.repo.server.client.cert: /app/config/reposerver/mtls/client.crt
  controller.repo.server.client.cert.key: /app/config/reposerver/mtls/client.key
  # controller.repo.server.ca.cert: /app/config/reposerver/mtls/ca.crt  # optional

  # For argocd-applicationset-controller:
  applicationsetcontroller.repo.server.client.cert: /app/config/reposerver/mtls/client.crt
  applicationsetcontroller.repo.server.client.cert.key: /app/config/reposerver/mtls/client.key
  # applicationsetcontroller.repo.server.ca.cert: /app/config/reposerver/mtls/ca.crt  # optional

  # For argocd-notifications-controller:
  notificationscontroller.repo.server.client.cert: /app/config/reposerver/mtls/client.crt
  notificationscontroller.repo.server.client.cert.key: /app/config/reposerver/mtls/client.key
  # notificationscontroller.repo.server.ca.cert: /app/config/reposerver/mtls/ca.crt  # optional
```

Alternatively, you can pass the flags directly in the container args:

```yaml
args:
- --repo-server-client-cert
- /app/config/reposerver/mtls/client.crt
- --repo-server-client-cert-key
- /app/config/reposerver/mtls/client.key
# Optional: verify repo-server with a custom CA
# - --repo-server-ca-cert
# - /app/config/reposerver/mtls/ca.crt
```

## Verifying mTLS is active

After deploying:

1. Check repo-server logs for the health-check certificate message when `--client-ca-path` is set:

   `Generated ephemeral health-check client certificate (CN=...)`

2. Attempt a connection from a pod without the client certificate — it should fail with a TLS error due to missing client authentication.
3. Connections from `argocd-server`/`argocd-application-controller`/`argocd-applicationset-controller` should succeed when configured with matching client cert/key.

## Troubleshooting

- Error: `--client-ca-path cannot be used when --disable-tls is enabled`
  - Remove `--disable-tls` (or unset `ARGOCD_REPO_SERVER_DISABLE_TLS`) when enabling mTLS.
- One of `--repo-server-client-cert` / `--repo-server-client-cert-key` missing
  - Provide both flags (or the corresponding environment variables) together.
- Custom CA for repo-server server certificate
  - Provide `--repo-server-ca-cert` on clients so they can verify the repo-server's server certificate.
