#!/bin/sh -xe

helm dependency update ./chart

AUTOGENMSG="# This is an auto-generated file. DO NOT EDIT"
echo "${AUTOGENMSG}" > ./chart/upstream.yaml

helm template argocd ./chart \
  --namespace argocd \
  --values ./chart/values.yaml \
  --no-hooks \
  >> ./chart/upstream_orig.yaml

sed -e 's/check inter 1s/check inter 3s/' ./chart/upstream_orig.yaml >> ./chart/upstream.yaml && rm ./chart/upstream_orig.yaml
sed -i 's/timeout server 30s/timeout server 6m/' ./chart/upstream.yaml
sed -i 's/timeout client 30s/timeout client 6m/' ./chart/upstream.yaml
