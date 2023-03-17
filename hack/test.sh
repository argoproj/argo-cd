#!/bin/bash
set -eux -o pipefail

which go-junit-report || go install github.com/jstemmer/go-junit-report@7fde4641acef5b92f397a8baf8309d1a45d608cc # v2.0.0

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

GODEBUG="tarinsecurepath=0,zipinsecurepath=0" go test $TEST_FLAGS -failfast $* 2>&1 | tee $TEST_RESULTS/test.out
