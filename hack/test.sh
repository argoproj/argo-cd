#!/bin/sh
set -eux

report() {
  set -xux
  go-junit-report --package-name com.github.argoproj.argo_cd < test.out > junit.xml
  xsltproc junit-noframes.xsl junit.xml > test.html
}

trap 'report' EXIT

go test -v -covermode=count -coverprofile=coverage.out $* 2>&1 | tee test.out
