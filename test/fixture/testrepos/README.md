# Test Repository Server Files

This directory contains configuration and cryptographic keys for running the E2E test Git server in a Docker container. The server provides Git repositories accessible via SSH, HTTP, and HTTPS.

## Overview

The test server runs in Docker (`argoproj/argo-cd-ci-builder:v1.0.0`) using goreman to manage three processes: sshd (port 2222), nginx (ports 9080, 9443-9445), and fcgiwrap (Git HTTP backend).

## Files

### `start-git.sh`

Starts the Docker container with the test Git server. Mounts the current directory and `ARGOCD_E2E_DIR` (default: `/tmp/argo-e2e`), exposing ports for SSH and HTTP/HTTPS access.

### `start-helm-registry.sh` / `start-authenticated-helm-registry.sh`

Start Helm registries for testing Helm chart functionality, with and without authentication.

### `Procfile`

Defines processes managed by goreman:

- **sshd**: Copies fixed SSH host keys to `/etc/ssh/` before starting on port 2222
- **fcgiwrap**: FastCGI wrapper for Git HTTP backend
- **nginx**: Web server for HTTP/HTTPS Git access

### SSH Keys

#### SSH Host Keys

- `ssh_host_rsa_key`, `ssh_host_ecdsa_key`, `ssh_host_ed25519_key` (with `.pub` files)

By copying pre-generated keys before starting sshd, the same keys are used every time, keeping the `ssh_known_hosts` the same on restarts.

#### `ssh_known_hosts`

Contains SSH host key fingerprints for:

- `[localhost]:2222` - Local development/testing
- `[argocd-e2e-server]:2222` - Remote/in-cluster testing

Both entries have identical fingerprints since they use the same keys from this directory.

#### `id_rsa.pub`

Public key added to `~/.ssh/authorized_keys` in the container for SSH authentication.

### `nginx.conf`

Provides HTTP (9080), HTTPS (9443/9444/9445), and Helm repository access. Proxies Git operations to git-http-backend via fcgiwrap.

### `sudoers.conf`

Sudo configuration allowing test users to start services with elevated privileges.

## Regenerating SSH Keys

If you need to regenerate the SSH host keys:

### 1. Generate New Keys

```bash
cd test/fixture/testrepos

# Generate RSA key
ssh-keygen -t rsa -b 2048 -f ssh_host_rsa_key -N "" -C "root@argocd-e2e"

# Generate ECDSA key
ssh-keygen -t ecdsa -f ssh_host_ecdsa_key -N "" -C "root@argocd-e2e"

# Generate Ed25519 key
ssh-keygen -t ed25519 -f ssh_host_ed25519_key -N "" -C "root@argocd-e2e"
```

### 2. Update ssh_known_hosts

```bash
# Start temporary sshd with new keys
sudo mkdir -p /tmp/test-sshd
sudo cp ssh_host_*_key* /tmp/test-sshd/
sudo chmod 600 /tmp/test-sshd/ssh_host_*_key

sudo /usr/sbin/sshd -p 2222 \
  -h /tmp/test-sshd/ssh_host_rsa_key \
  -h /tmp/test-sshd/ssh_host_ecdsa_key \
  -h /tmp/test-sshd/ssh_host_ed25519_key \
  -D &
SSHD_PID=$!

# Scan to get new fingerprints
ssh-keyscan -p 2222 localhost > ssh_known_hosts.tmp

# Stop temporary sshd
sudo kill $SSHD_PID
sudo rm -rf /tmp/test-sshd

# Create new ssh_known_hosts with both localhost and argocd-e2e-server entries
cat > ssh_known_hosts << 'EOF'
# localhost:2222 SSH-2.0-OpenSSH_X.Xp1
EOF
cat ssh_known_hosts.tmp >> ssh_known_hosts
echo "# For in-cluster tests" >> ssh_known_hosts
sed 's/\[localhost\]/[argocd-e2e-server]/g' ssh_known_hosts.tmp >> ssh_known_hosts

rm ssh_known_hosts.tmp
```

### 3. Verify

```bash
# Start the test server
./test/fixture/testrepos/start-git.sh

# In another terminal, test SSH connection
ssh -p 2222 -o UserKnownHostsFile=test/fixture/testrepos/ssh_known_hosts root@localhost
```

## Related Documentation

- [TLS Certificates](../certs/README.md) - HTTPS certificates (separate from SSH keys)
- [Remote Testing](../../remote/README.md) - Running tests against remote clusters
