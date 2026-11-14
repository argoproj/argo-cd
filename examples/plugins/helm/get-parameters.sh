#!/bin/sh

yq e -o=json values.yaml | jq '[{
  name: "helm-parameters",
  title: "Helm Parameters",
  collectionType: "map",
  map: [leaf_paths as $path | {"key": $path | join("."), "value": getpath($path)|tostring}] | from_entries
}]'
