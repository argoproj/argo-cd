#! /usr/bin/env bash

# call login.sh to be sure we are logged
script_dir=$(dirname "$0")
"$script_dir/login.sh"

# Fetch the list of application names
echo "fetching apps ..."
app_list=$(argocd app list -o name)

# Loop through each application and delete it
for app_name in $app_list; do
  echo "deleting '$app_name' ..."
  argocd app delete "$(basename "$app_name")" --cascade
done

# todo delete also the namespaces
# https://github.com/argoproj/argo-cd/issues/4435
# https://github.com/argoproj/argo-cd/issues/7875
