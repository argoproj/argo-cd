#!/usr/bin/env sh

# Usage: ./add-oras-checksums.sh 1.2.0  # use the desired version

wget "https://github.com/oras-project/oras/releases/download/v$1/oras_$1_checksums.txt"

while IFS="" read -r line || [ -n "$line" ]
do
  filename=$(echo "$line" | awk -F ' ' '{print $2}' | sed "s#v$1#$1#")
  test "${line#*windows}" == "$line" && echo "$line" | sed "s#v$1#$1#" > "$filename.sha256"
done < oras_$1_checksums.txt

rm oras_$1_checksums.txt
