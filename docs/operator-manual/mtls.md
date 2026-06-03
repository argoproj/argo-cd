# Mutual TLS (mTLS) for repo-server

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
* `ARGOCD_SERVER_REPO_SERVER_CLIENT_CERT`
* `ARGOCD_SERVER_REPO_SERVER_CLIENT_CERT_KEY`
* `ARGOCD_SERVER_REPO_SERVER_CA_CERT`

#### argocd-application-controller
* `ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER_CLIENT_CERT`
* `ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER_CLIENT_CERT_KEY`
* `ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER_CA_CERT`

#### argocd-applicationset-controller
* `ARGOCD_APPLICATIONSET_CONTROLLER_REPO_SERVER_CLIENT_CERT`
* `ARGOCD_APPLICATIONSET_CONTROLLER_REPO_SERVER_CLIENT_CERT_KEY`
* `ARGOCD_APPLICATIONSET_CONTROLLER_REPO_SERVER_CA_CERT`

> [!IMPORTANT]
> Both `--repo-server-client-cert` and `--repo-server-client-cert-key` must be provided together. If you provide one without the other, the component will fail validation at startup.

## Deployment using Kubernetes Secrets

It is recommended to use Kubernetes Secrets to mount the certificates and keys into the Argo CD pods.

1. Create a secret with the CA, client certificate, and key.
2. Mount the secret as a volume in the `argocd-repo-server`, `argocd-server`, `argocd-application-controller`, and `argocd-applicationset-controller` deployments.
3. Update the container arguments or environment variables to point to the mounted files.

Example Secret containing repo-server client CA and client cert/key:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: argocd-reposerver-mtls
type: Opaque
data:
  # Base64-encoded files
  client-ca.crt: <BASE64_CA_PEM>
  client.crt: <BASE64_CLIENT_CERT_PEM>
  client.key: <BASE64_CLIENT_KEY_PEM>
```

Example `argocd-repo-server` (enable mTLS):
```yaml
containers:
- name: argocd-repo-server
  volumeMounts:
  - name: reposerver-tls
    mountPath: /app/config/reposerver/tls
  args:
  - --client-ca-path
  - /app/config/reposerver/tls/client-ca.crt
volumes:
- name: reposerver-tls
  secret:
    secretName: argocd-reposerver-mtls
    items:
    - key: client-ca.crt
      path: client-ca.crt
    # If you also manage the server's TLS cert/key via the same Secret, include:
    # - key: tls.crt
    #   path: tls.crt
    # - key: tls.key
    #   path: tls.key
```

Example `argocd-server` (use client cert to talk to repo-server):
```yaml
containers:
- name: argocd-server
  volumeMounts:
  - name: reposerver-tls
    mountPath: /app/config/reposerver/tls
  args:
  - --repo-server-client-cert
  - /app/config/reposerver/tls/client.crt
  - --repo-server-client-cert-key
  - /app/config/reposerver/tls/client.key
  # Optional: verify repo-server with a custom CA
  # - --repo-server-ca-cert
  # - /app/config/reposerver/tls/ca.crt
volumes:
- name: reposerver-tls
  secret:
    secretName: argocd-reposerver-mtls
    items:
    - key: client.crt
      path: client.crt
    - key: client.key
      path: client.key
    # Optional, when repo-server uses a custom CA for its server cert
    # - key: ca.crt
    #   path: ca.crt
```

Example `argocd-application-controller` configuration is analogous to `argocd-server` (use the same flags/paths and Secret mount).

Example `argocd-applicationset-controller` (use client cert to talk to repo-server):
```yaml
containers:
- name: argocd-applicationset-controller
  volumeMounts:
  - name: reposerver-tls
    mountPath: /app/config/reposerver/tls
  args:
  - --repo-server-client-cert
  - /app/config/reposerver/tls/client.crt
  - --repo-server-client-cert-key
  - /app/config/reposerver/tls/client.key
  # Optional: verify repo-server with a custom CA
  # - --repo-server-ca-cert
  # - /app/config/reposerver/tls/ca.crt
volumes:
- name: reposerver-tls
  secret:
    secretName: argocd-reposerver-mtls
    items:
    - key: client.crt
      path: client.crt
    - key: client.key
      path: client.key
    # Optional, when repo-server uses a custom CA for its server cert
    # - key: ca.crt
    #   path: ca.crt
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
