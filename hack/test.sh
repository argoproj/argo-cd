#!/bin/bash
set -eux -o pipefail

# make sure apiclient does not depend on packr
which godepgraph || go get github.com/kisielk/godepgraph
if godepgraph -s github.com/argoproj/argo-cd/pkg/apiclient | grep packr; then
  echo apiclient package should not depend on packr
  exit 1
fi

TEST_RESULTS=${TEST_RESULTS:-test-results}

mkdir -p $TEST_RESULTS

report() {
  set -eux -o pipefail

  go-junit-report < $TEST_RESULTS/test.out > $TEST_RESULTS/junit.xml
}

trap 'report' EXIT

go test -v $* 2>&1 | tee $TEST_RESULTS/test.out
