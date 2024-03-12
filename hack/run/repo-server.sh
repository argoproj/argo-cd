#!/bin/sh
export ARGOCD_BINARY_NAME=argocd-repo-server
if [ "$BIN_MODE" = 'true' ]; then
	COMMAND=./dist/argocd
else
	COMMAND='go run ./cmd/main.go'
fi

export ARGOCD_GNUPGHOME=${ARGOCD_GNUPGHOME:-/tmp/argocd-local/gpg/keys}
export ARGOCD_PLUGINSOCKFILEPATH=${ARGOCD_PLUGINSOCKFILEPATH:-./test/cmp}
export ARGOCD_GPG_DATA_PATH=${ARGOCD_GPG_DATA_PATH:-/tmp/argocd-local/gpg/source}
export ARGOCD_TLS_DATA_PATH=${ARGOCD_TLS_DATA_PATH:-/tmp/argocd-local/tls}
export ARGOCD_SSH_DATA_PATH=${ARGOCD_SSH_DATA_PATH:-/tmp/argocd-local/ssh}
export ARGOCD_GPG_ENABLED=${ARGOCD_GPG_ENABLED:-false}

$COMMAND --loglevel debug --port ${ARGOCD_E2E_REPOSERVER_PORT:-8081} --redis localhost:${ARGOCD_E2E_REDIS_PORT:-6379} --otlp-address=${ARGOCD_OTLP_ADDRESS} $*
