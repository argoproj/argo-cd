#!/bin/bash
set -e
FILE=$(echo "$GIT_SSH_COMMAND" | grep -oP '\-i \K[/\w]+')
GIT_SSH_CRED_FILE_SHA=$(sha256sum ${FILE})
echo "{\"kind\": \"ConfigMap\", \"apiVersion\": \"v1\", \"metadata\": { \"name\": \"$ARGOCD_APP_NAME\", \"namespace\": \"$ARGOCD_APP_NAMESPACE\", \"annotations\": {\"GitSSHCommand\":\"$GIT_SSH_COMMAND\", \"GitSSHCredsFileSHA\":\"$GIT_SSH_CRED_FILE_SHA\"}}}"