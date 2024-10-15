#!/bin/bash
set -eux -o pipefail

# Ensure go-junit-report is installed
which go-junit-report || go install github.com/jstemmer/go-junit-report@latest

# Ensure DIST_DIR is defined and set to the directory where gotestsum should be installed
DIST_DIR=${DIST_DIR:-/go/src/github.com/argoproj/argo-cd/dist}

# Ensure gotestsum is installed in DIST_DIR
if [ ! -x "${DIST_DIR}/gotestsum" ]; then
    echo "gotestsum not found in DIST_DIR, installing..."
    go install gotest.tools/gotestsum@latest
    mkdir -p "${DIST_DIR}"
    cp "$(which gotestsum)" "${DIST_DIR}/gotestsum"
else
    echo "gotestsum is already installed in DIST_DIR."
fi

# Set default values for TEST_RESULTS and TEST_FLAGS
TEST_RESULTS=${TEST_RESULTS:-test-results}
TEST_FLAGS=${TEST_FLAGS:-}

# Append parallelism and verbosity flags if set
if [ "${ARGOCD_TEST_PARALLELISM:-}" != "" ]; then
    TEST_FLAGS="$TEST_FLAGS -p $ARGOCD_TEST_PARALLELISM"
fi
if [ "${ARGOCD_TEST_VERBOSE:-}" != "" ]; then
    TEST_FLAGS="$TEST_FLAGS -v"
fi

# Create the test results directory if it does not exist
mkdir -p $TEST_RESULTS

# Run gotestsum with the specified parameters
GODEBUG="tarinsecurepath=0,zipinsecurepath=0" ${DIST_DIR}/gotestsum --rerun-fails-report=rerunreport.txt --junitfile=$TEST_RESULTS/junit.xml --format=testname --rerun-fails="$RERUN_FAILS" --packages="$PACKAGES" -- -cover $TEST_FLAGS $*
