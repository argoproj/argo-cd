#!/bin/sh
SCRIPTPATH="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
export ARGOCD_BINARY_NAME=argocd-server
if [ "$BIN_MODE" = "true" ]; then
	COMMAND=./dist/argocd
else
	COMMAND='go run ./cmd/main.go'
fi
. $SCRIPTPATH/include.sh

export ARGOCD_TLS_DATA_PATH=${ARGOCD_TLS_DATA_PATH:-/tmp/argocd-local/tls}
export ARGOCD_SSH_DATA_PATH=${ARGOCD_SSH_DATA_PATH:-/tmp/argocd-local/ssh}

$COMMAND --loglevel debug --redis localhost:${ARGOCD_E2E_REDIS_PORT:-6379} --disable-auth=${ARGOCD_E2E_DISABLE_AUTH:-'true'} --insecure --dex-server http://localhost:${ARGOCD_E2E_DEX_PORT:-5556} --repo-server localhost:${ARGOCD_E2E_REPOSERVER_PORT:-8081} --port ${ARGOCD_E2E_APISERVER_PORT:-8080} --otlp-address=${ARGOCD_OTLP_ADDRESS} --application-namespaces=${ARGOCD_APPLICATION_NAMESPACES:-''}
