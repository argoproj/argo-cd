#!/bin/sh -xe

helm2 dependency update ./chart --skip-refresh

# This step is necessary because we do not want the helm tests to be included
templates=$(tar -tf ./chart/charts/redis-ha-*.tgz | grep 'redis-ha/templates/redis-.*.yaml')
helm_execute=""
for tmpl in ${templates}; do
  helm_execute="${helm_execute} -x charts/${tmpl}"
done

AUTOGENMSG="# This is an auto-generated file. DO NOT EDIT"
echo "${AUTOGENMSG}" > ./chart/upstream.yaml

helm2 template ./chart \
  --name argocd \
  --namespace argocd \
  --values ./chart/values.yaml \
  ${helm_execute} \
  >> ./chart/upstream_orig.yaml

sed -e 's/check inter 1s/check inter 3s/' ./chart/upstream_orig.yaml >> ./chart/upstream.yaml && rm ./chart/upstream_orig.yaml
sed -i 's/timeout server 30s/timeout server 6m/' ./chart/upstream.yaml
sed -i 's/timeout client 30s/timeout client 6m/' ./chart/upstream.yaml
