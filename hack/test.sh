#!/bin/bash
set -eux -o pipefail

TEST_RESULTS=${TEST_RESULTS:-test-results}

mkdir -p $TEST_RESULTS

report() {
  set -eux -o pipefail

  go-junit-report --package-name com.github.argoproj.argo_cd < $TEST_RESULTS/test.out > $TEST_RESULTS/junit.xml
}

trap 'report' EXIT

go test -v $* 2>&1 | tee $TEST_RESULTS/test.out
