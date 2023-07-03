#! /usr/bin/env bash

# call login.sh to be sure we are logged
script_dir=$(dirname "$0")
"$script_dir/login.sh"

# Fetch the list of application namespaces
echo "fetching in-cluster app namespaces..."
app_namespaces=$(argocd app list --output json | jq -r 'map(select(.spec.destination.server == "https://kubernetes.default.svc")) | .[].spec.destination.namespace')

echo -e "app-namespaces:\n$app_namespaces"

