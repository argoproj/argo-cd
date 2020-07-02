#!/bin/sh

MANDATORY_COMPONENTS="dex redis ui git-server dev-mounter"

case "$1" in
"server")
	ADDITIONAL_COMPONENTS="repo-server controller"
	;;
"repo-server")
	ADDITIONAL_COMPONENTS="api-server controller"
	;;
"controller")
	ADDITIONAL_COMPONENTS="api-server repo-server"
	;;
*)
	echo "USAGE: $0 <server|repo-server|controller>" >&2
	exit 1
	;;
esac

export ARGOCD_START="${ADDITIONAL_COMPONENTS} ${MANDATORY_COMPONENTS}"
make start-local

