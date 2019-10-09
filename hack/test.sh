#!/bin/bash
set -eux -o pipefail

mkdir -p test-results

report() {
  set -eux -o pipefail

  go-junit-report --package-name com.github.argoproj.argo_cd < test-results/test.out > test-results/junit.xml
  xsltproc junit-noframes.xsl test-results/junit.xml > test-results/test.html
}

trap 'report' EXIT

go test -v -covermode=count -coverprofile=coverage.out $* 2>&1 | tee test-results/test.out
