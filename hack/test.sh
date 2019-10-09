#!/bin/bash
set -eux -o pipefail

report() {
  set -eux -o pipefail

  go-junit-report --package-name com.github.argoproj.argo_cd < test.out > junit.xml
  xsltproc junit-noframes.xsl junit.xml > test.html
}

trap 'report' EXIT

go test -v $* 2>&1 | tee test.out
