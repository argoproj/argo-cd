#!/bin/sh
SCRIPTPATH="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
. $SCRIPTPATH/include.sh

export ARGOCD_TLS_DATA_PATH=${ARGOCD_TLS_DATA_PATH:-/tmp/argocd-local/tls}
export ARGOCD_SSH_DATA_PATH=${ARGOCD_SSH_DATA_PATH:-/tmp/argocd-local/ssh}

if [ "$ARGOCD_E2E_TEST" != "true" ]; then
       	go run ./hack/dev-mounter/main.go --configmap argocd-ssh-known-hosts-cm=${ARGOCD_SSH_DATA_PATH} --configmap argocd-tls-certs-cm=${ARGOCD_TLS_DATA_PATH} --configmap argocd-gpg-keys-cm=${ARGOCD_GPG_DATA_PATH:-/tmp/argocd-local/gpg/source}
fi
