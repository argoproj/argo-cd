#!/bin/bash

mkdir dir1
# Move all files and directories except dir1 and prepare.sh
for item in *; do
  if [[ "$item" != "dir1" && "$item" != "prepare.sh" ]]; then
    mv "$item" dir1/
  fi
done
git rm -r pkg specs agent internal go.* Dockerfile LICENSE OWNERS README.md docs hack codecov.yml sonar-project.properties .github/ .golangci.yaml .gitignore
rm dir1/sonar-project.properties dir1/codecov.yml
mv dir1/ gitops-engine
git checkout -b migrate
git add gitops-engine
git add prepare.sh
git commit --signoff -m "prepare repo for migration to ArgoCD repo"
