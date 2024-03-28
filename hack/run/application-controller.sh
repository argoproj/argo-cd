#!/bin/sh
SCRIPTPATH="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
export ARGOCD_BINARY_NAME=argocd-application-controller
if [ "$BIN_MODE" = "true" ]; then
	COMMAND=./dist/argocd
else
	COMMAND='go run ./cmd/main.go'
fi
. $SCRIPTPATH/include.sh

$COMMAND --loglevel debug --redis localhost:${ARGOCD_E2E_REDIS_PORT:-6379} --repo-server localhost:${ARGOCD_E2E_REPOSERVER_PORT:-8081} --otlp-address=${ARGOCD_OTLP_ADDRESS} --application-namespaces=${ARGOCD_APPLICATION_NAMESPACES:-''} --server-side-diff-enabled=${ARGOCD_APPLICATION_CONTROLLER_SERVER_SIDE_DIFF:-'false'}
