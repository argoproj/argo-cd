#!/bin/sh
echo "
[{
    \"name\": \"helm-parameters\",
    \"title\": \"Helm Parameters\",
    \"collectionType\": \"map\",
    \"map\": $(yq e -o=json values.yaml | jq '[leaf_paths as $path | {"key": $path | join("."), "value": getpath($path)|tostring}] | from_entries')
  }]" | tr -d "\n"

