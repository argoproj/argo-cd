#!/bin/bash
set -eux -o pipefail

# make sure apiclient does not depend on packr
which godepgraph || go get github.com/kisielk/godepgraph
which go-junit-report || go get github.com/jstemmer/go-junit-report

export GO111MODULE=off
if godepgraph -s github.com/argoproj/argo-cd/pkg/apiclient | grep packr; then
  echo apiclient package should not depend on packr
  exit 1
fi
unset GO111MODULE

TEST_RESULTS=${TEST_RESULTS:-test-results}
TEST_FLAGS=

if test "${ARGOCD_TEST_PARALLELISM:-}" != ""; then
	TEST_FLAGS="$TEST_FLAGS -p $ARGOCD_TEST_PARALLELISM"
fi
if test "${ARGOCD_TEST_VERBOSE:-}" != ""; then
	TEST_FLAGS="$TEST_FLAGS -v"
fi

mkdir -p $TEST_RESULTS

report() {
  set -eux -o pipefail

  go-junit-report < $TEST_RESULTS/test.out > $TEST_RESULTS/junit.xml
}

trap 'report' EXIT

go test $TEST_FLAGS -failfast $* 2>&1 | tee $TEST_RESULTS/test.out
