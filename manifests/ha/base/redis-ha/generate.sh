#!/bin/sh -xe

helm dependency update ./chart

AUTOGENMSG="# This is an auto-generated file. DO NOT EDIT"
echo "${AUTOGENMSG}" > ./chart/upstream.yaml

helm template argocd ./chart \
  --namespace argocd \
  --values ./chart/values.yaml \
  --no-hooks \
  >> ./chart/upstream_orig.yaml

sed -E 's/^([[:space:]]){8}sentinel replaceme argocd/    bind/' ./chart/upstream_orig.yaml >> ./chart/upstream.yaml && rm ./chart/upstream_orig.yaml
