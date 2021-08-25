#!/usr/bin/env bash

declare -a services=("controller" "api-server" "redis" "repo-server" "ui")

EXCLUDE=$exclude

declare -a servicesToRun=()

if [ "$EXCLUDE" != "" ]; then
    servicesToExclude=($(echo "$EXCLUDE" | tr ',' '\n'))

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
          servicesToRun+=($element)
        fi
    done

fi

command="goreman start "

for element in "${servicesToRun[@]}"
do
  command+=$element
  command+=" "
done

eval $command