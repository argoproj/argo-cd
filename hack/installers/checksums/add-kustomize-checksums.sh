#!/usr/bin/env sh

# Usage: ./add-kustomize-checksums.sh 4.5.7  # use the desired version

set -e

wget "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv$1/checksums.txt"

while IFS="" read -r line || [ -n "$line" ]
do
  filename=$(echo "$line" | awk -F ' ' '{print $2}' | sed "s#v$1#$1#")
  test "${line#*windows}" == "$line" && echo "$line" | sed "s#v$1#$1#" > "$filename.sha256"
done < checksums.txt

rm checksums.txt
