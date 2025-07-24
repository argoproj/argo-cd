#!/usr/bin/env bash
export HELM_EXPERIMENTAL_OCI=1
docker run -p 5001:5000 --rm --name authed-registry -v $(pwd)/test/fixture/testrepos/.oci-htpasswd:/etc/docker/registry/auth.htpasswd \
-e REGISTRY_AUTH="{htpasswd: {realm: localhost, path: /etc/docker/registry/auth.htpasswd}}" \
registry