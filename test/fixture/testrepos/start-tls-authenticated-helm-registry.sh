#!/usr/bin/env bash
docker run -p 5002:5000 --rm --name tls-authed-registry \
	-v "$(pwd)"/test/fixture/testrepos/.oci-htpasswd:/etc/docker/registry/auth.htpasswd:ro \
	-v "$(pwd)"/test/fixture/certs/argocd-test-server.crt:/certs/registry.crt:ro \
	-v "$(pwd)"/test/fixture/certs/argocd-test-server.key:/certs/registry.key:ro \
	-e REGISTRY_AUTH="{htpasswd: {realm: localhost, path: /etc/docker/registry/auth.htpasswd}}" \
	-e REGISTRY_HTTP_TLS_CERTIFICATE=/certs/registry.crt \
	-e REGISTRY_HTTP_TLS_KEY=/certs/registry.key \
	registry