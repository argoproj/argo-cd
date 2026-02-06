#!/bin/bash
set -e
# Extract the file path after -i from GIT_SSH_COMMAND
FILE=$(echo "$GIT_SSH_COMMAND" | sed -n 's/.*-i \([^ ]*\).*/\1/p')
GIT_SSH_CRED_FILE_SHA=$(sha256sum "${FILE}")
echo "{\"kind\": \"ConfigMap\", \"apiVersion\": \"v1\", \"metadata\": { \"name\": \"$ARGOCD_APP_NAME\", \"namespace\": \"$ARGOCD_APP_NAMESPACE\", \"annotations\": {\"GitSSHCommand\":\"$GIT_SSH_COMMAND\", \"GitSSHCredsFileSHA\":\"$GIT_SSH_CRED_FILE_SHA\"}}}"
