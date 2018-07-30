#!/bin/sh

IMAGE_NAMESPACE=${IMAGE_NAMESPACE:='argoproj'}
IMAGE_TAG=${IMAGE_TAG:='latest'}

for i in "$(ls manifests/components/*.yaml)"; do
    sed -i '' 's@\( image: \(.*\)/\(argocd-.*\):.*\)@ image: '"${IMAGE_NAMESPACE}"'/\3:'"${IMAGE_TAG}"'@g' $i
done

echo "# This is an auto-generated file. DO NOT EDIT" > manifests/install.yaml
cat manifests/components/*.yaml >> manifests/install.yaml
