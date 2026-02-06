#!/usr/bin/env bash

declare -a services=("controller" "api-server" "redis" "repo-server" "cmp-server" "ui" "applicationset-controller" "commit-server" "notification" "dex" "git-server" "helm-registry" "dev-mounter")

EXCLUDE="$exclude"

declare -a servicesToRun=()

if [ "$EXCLUDE" != "" ]; then
    # Split services list by ',' character
    readarray -t servicesToExclude < <(tr ',' '\n' <<< "$EXCLUDE")

    # Find subset of items from services array that not include servicesToExclude items
    for element in "${services[@]}"
    do
        found=false
        for excludedSvc in "${servicesToExclude[@]}"
        do
          if [[ "$excludedSvc" == "$element" ]]; then
            found=true
          fi
        done
        if [[ "$found" == false ]]; then
          servicesToRun+=("$element")
        fi
    done
fi

command=("goreman" "start")

for element in "${servicesToRun[@]}"
do
  command+=("$element")
done

"${command[@]}"
