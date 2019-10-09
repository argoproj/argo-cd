#!/bin/bash
set -eux -o pipefail

mkdir -p tests-results

report() {
  set -eux -o pipefail

  go-junit-report --package-name com.github.argoproj.argo_cd < tests-results/test.out > tests-results/junit.xml
  xsltproc junit-noframes.xsl tests-results/junit.xml > tests-results/test.html
}

trap 'report' EXIT

go test -v -covermode=count -coverprofile=coverage.out $* 2>&1 | tee tests-results/test.out
