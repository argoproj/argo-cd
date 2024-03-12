#!/bin/sh
SCRIPTPATH="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
export ARGOCD_BINARY_NAME=argocd-notifications
if [ "$BIN_MODE" = 'true' ]; then
	COMMAND=./dist/argocd
else
	COMMAND='go run ./cmd/main.go'
fi
export ARGOCD_TLS_DATA_PATH=${ARGOCD_TLS_DATA_PATH:-/tmp/argocd-local/tls}
$COMMAND --loglevel debug --application-namespaces=${ARGOCD_APPLICATION_NAMESPACES:-''} --self-service-notification-enabled=${ARGOCD_NOTIFICATION_CONTROLLER_SELF_SERVICE_NOTIFICATION_ENABLED:-'false'}
