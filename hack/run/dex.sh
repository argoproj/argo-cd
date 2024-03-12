#!/bin/sh
SCRIPTPATH="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
. $SCRIPTPATH/include.sh
go run github.com/argoproj/argo-cd/v2/cmd gendexcfg -o `pwd`/dist/dex.yaml && (test -f dist/dex.yaml || { echo 'Failed to generate dex configuration'; exit 1; }) && docker run --rm -p ${ARGOCD_E2E_DEX_PORT:-5556}:${ARGOCD_E2E_DEX_PORT:-5556} -v `pwd`/dist/dex.yaml:/dex.yaml ghcr.io/dexidp/dex:$(grep "image: ghcr.io/dexidp/dex" manifests/base/dex/argocd-dex-server-deployment.yaml | cut -d':' -f3) dex serve /dex.yaml"

