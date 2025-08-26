#!/bin/bash

mkdir dir1
mv *[^dir1prepare.sh]* dir1/
git rm -r pkg specs agent internal go.* Dockerfile LICENSE OWNERS README.md docs hack codecov.yml Makefile sonar-project.properties .github/ .golangci.yaml .gitignore
rm dir1/sonar-project.properties dir1/codecov.yml
mv dir1/ gitops-engine
git checkout -b migrate
git add gitops-engine
git add prepare.sh
git commit --signoff -m "prepare repo for migration to ArgoCD repo"
git push --set-upstream origin migrate