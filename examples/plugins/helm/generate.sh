#!/bin/sh

ARGUMENTS=$(echo "$ARGOCD_APP_PARAMETERS" | jq -r '.[] | select(.name == "values-files").array | .[] | "--values=" + .')
PARAMETERS=$(echo "$ARGOCD_APP_PARAMETERS" | jq -r '.[] | select(.name == "helm-parameters").map | to_entries | map("\(.key)=\(.value)") | .[] | "--set=" + .')

echo ". $ARGUMENTS $PARAMETERS" | xargs helm template
