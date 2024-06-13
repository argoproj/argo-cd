#!/bin/sh
export ARGOCD_BINARY_NAME=argocd-applicationset-controller
SCRIPTPATH="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
. $SCRIPTPATH/include.sh
if [ "$BIN_MODE" = 'true' ]; then
	COMMAND=./dist/argocd
else
	COMMAND='go run ./cmd/main.go'
fi
export ARGOCD_TLS_DATA_PATH=${ARGOCD_TLS_DATA_PATH:-/tmp/argocd-local/tls}
export ARGOCD_SSH_DATA_PATH=${ARGOCD_SSH_DATA_PATH:-/tmp/argocd-local/ssh}

$COMMAND --loglevel debug --metrics-addr localhost:12345 --probe-addr localhost:12346 --argocd-repo-server localhost:${ARGOCD_E2E_REPOSERVER_PORT:-8081}
