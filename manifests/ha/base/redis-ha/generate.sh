#!/bin/sh -xe

helm dependency update ./chart

AUTOGENMSG="# This is an auto-generated file. DO NOT EDIT"
echo "${AUTOGENMSG}" > ./chart/upstream.yaml

helm template argocd ./chart \
  --namespace argocd \
  --values ./chart/values.yaml \
  --no-hooks \
  >> ./chart/upstream.yaml
