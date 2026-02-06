#!/usr/bin/env bash

set -e
set -o pipefail

# Fail on high-or-greater vulnerabilities.
snyk test --all-projects --exclude=docs,site,ui-test --org=argoproj --severity-threshold=high --policy-path=.snyk --sarif-file-output=/tmp/argocd-test.sarif
snyk iac test manifests/install.yaml                 --org=argoproj --severity-threshold=high --policy-path=.snyk --sarif-file-output=/tmp/argocd-iac-test-install.sarif
snyk iac test manifests/namespace-install.yaml       --org=argoproj --severity-threshold=high --policy-path=.snyk --sarif-file-output=/tmp/argocd-iac-test-namespace-install.sarif
