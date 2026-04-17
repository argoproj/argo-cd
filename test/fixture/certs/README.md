# Testing certificates

This directory contains all TLS certificates used for testing ArgoCD, including
the E2E tests. It also contains the CA certificate and key used for signing the
certificates.

## Certificate Files

- `argocd-test-ca.crt` and `argocd-test-ca.key` - The CA certificate and key
- `argocd-test-client.crt` and `argocd-test-client.key` - Client certificate/key pair for mTLS testing
- `argocd-test-server.crt` and `argocd-test-server.key` - Server certificate/key pair (CN=localhost, SANs: localhost, argocd-e2e-server, 127.0.0.1)
- `argocd-e2e-server.crt` and `argocd-e2e-server.key` - Server certificate for remote E2E tests (CN=argocd-e2e-server)

All keys have no passphrase. All certs are valid for 100 years.

**Do not use these certs for anything other than Argo CD tests.**

## Regenerating Certificates

If you need to regenerate the certificates (e.g., for compliance with newer TLS standards), run the following commands from this directory:

### 1. Generate CA Certificate

```bash
openssl genrsa -out argocd-test-ca.key 4096

openssl req -new -x509 -days 36500 -key argocd-test-ca.key \
    -out argocd-test-ca.crt \
    -subj "/CN=ArgoCD Test CA" \
    -addext "basicConstraints=critical,CA:TRUE" \
    -addext "keyUsage=critical,keyCertSign,cRLSign"
```

### 2. Generate Server Certificate (for localhost testing)

```bash
openssl genrsa -out argocd-test-server.key 4096

openssl req -new -key argocd-test-server.key \
    -out argocd-test-server.csr \
    -subj "/CN=localhost"

cat > server_ext.cnf << 'EOF'
basicConstraints=CA:FALSE
keyUsage=critical,digitalSignature,keyEncipherment
extendedKeyUsage=serverAuth
subjectAltName=DNS:localhost,DNS:argocd-e2e-server,IP:127.0.0.1
EOF

openssl x509 -req -in argocd-test-server.csr \
    -CA argocd-test-ca.crt -CAkey argocd-test-ca.key \
    -CAcreateserial -out argocd-test-server.crt \
    -days 36500 -sha256 \
    -extfile server_ext.cnf

rm -f server_ext.cnf argocd-test-server.csr argocd-test-ca.srl
```

### 3. Generate Client Certificate (for mTLS testing)

```bash
openssl genrsa -out argocd-test-client.key 4096

openssl req -new -key argocd-test-client.key \
    -out argocd-test-client.csr \
    -subj "/CN=ArgoCD Test Client"

cat > client_ext.cnf << 'EOF'
basicConstraints=CA:FALSE
keyUsage=critical,digitalSignature,keyEncipherment
extendedKeyUsage=clientAuth
EOF

openssl x509 -req -in argocd-test-client.csr \
    -CA argocd-test-ca.crt -CAkey argocd-test-ca.key \
    -CAcreateserial -out argocd-test-client.crt \
    -days 36500 -sha256 \
    -extfile client_ext.cnf

rm -f client_ext.cnf argocd-test-client.csr argocd-test-ca.srl
```

### 4. Generate E2E Server Certificate (for remote testing)

```bash
openssl genrsa -out argocd-e2e-server.key 4096

openssl req -new -key argocd-e2e-server.key \
    -out argocd-e2e-server.csr \
    -subj "/CN=argocd-e2e-server"

cat > e2e_server_ext.cnf << 'EOF'
basicConstraints=CA:FALSE
keyUsage=critical,digitalSignature,keyEncipherment
extendedKeyUsage=serverAuth
subjectAltName=DNS:argocd-e2e-server,DNS:localhost,IP:127.0.0.1
EOF

openssl x509 -req -in argocd-e2e-server.csr \
    -CA argocd-test-ca.crt -CAkey argocd-test-ca.key \
    -CAcreateserial -out argocd-e2e-server.crt \
    -days 36500 -sha256 \
    -extfile e2e_server_ext.cnf

rm -f e2e_server_ext.cnf argocd-e2e-server.csr argocd-test-ca.srl
```

### Verify Certificates

After regenerating, verify the certificate chain:

```bash
openssl verify -CAfile argocd-test-ca.crt argocd-test-server.crt
openssl verify -CAfile argocd-test-ca.crt argocd-test-client.crt
openssl verify -CAfile argocd-test-ca.crt argocd-e2e-server.crt
```
