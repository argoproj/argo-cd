!#/bin/bash

mkdir gitops-engine
find . -type d \( -path ./gitops-engine -o -path ./.git -o -path ./.github -o -path ./.gitignore \) -prune -o -type f \( -path ./.gitignore -o -path ./.golangci.yaml -o -path ./codecov.yml -o -path ./sonar-project.properties -o -path . \) -prune -o -name '*' -maxdepth 1 -exec echo {} \;
git rm -r .github
git rm .gitignore
git rm .golangci.yaml
git rm codecov.yml
git rm sonar-project.properties
