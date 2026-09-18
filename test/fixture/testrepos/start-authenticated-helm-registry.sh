#!/usr/bin/env bash
docker run -p "${ARGOCD_E2E_OCI_REGISTRY_PORT:-5001}:5000" --rm --name authed-registry -v "$(pwd)"/test/fixture/testrepos/.oci-htpasswd:/etc/docker/registry/auth.htpasswd \
-e REGISTRY_AUTH="{htpasswd: {realm: localhost, path: /etc/docker/registry/auth.htpasswd}}" \
registry
