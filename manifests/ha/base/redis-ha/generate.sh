#!/bin/sh -xe

helm dependency update ./chart --skip-refresh

# This step is necessary because we do not want the helm tests to be included
templates=$(tar -tf ./chart/charts/redis-ha-*.tgz | grep 'redis-ha/templates/redis-.*.yaml')
helm_execute=""
for tmpl in ${templates}; do
  helm_execute="${helm_execute} -x charts/${tmpl}"
done

AUTOGENMSG="# This is an auto-generated file. DO NOT EDIT"
echo "${AUTOGENMSG}" > ./chart/upstream.yaml

helm template ./chart \
  --name argocd \
  --values ./chart/values.yaml \
  ${helm_execute} \
  >> ./chart/upstream.yaml
