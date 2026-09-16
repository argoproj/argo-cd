#!/usr/bin/env bash
docker run -p "${ARGOCD_E2E_HELM_REGISTRY_PORT:-5050}:5000" --rm --name registry registry
