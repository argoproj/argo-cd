#/bin/sh
SCRIPTPATH="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
export ARGOCD_BINARY_NAME=argocd-cmp-server
if [ "$ARGOCD_E2E_TEST" = 'true' ]; then
       exit 0
fi 
if [ "$BIN_MODE" = 'true' ]; then
	COMMAND=./dist/argocd
else
	COMMAND='go run ./cmd/main.go'
fi 
. $SCRIPTPATH/include.sh

export ARGOCD_PLUGINSOCKFILEPATH=${ARGOCD_PLUGINSOCKFILEPATH:-./test/cmp}
$COMMAND --config-dir-path ./test/cmp --loglevel debug --otlp-address=${ARGOCD_OTLP_ADDRESS}"
