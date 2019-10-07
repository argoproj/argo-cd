#!/bin/sh
set -eux

trap 'go-junit-report < test-e2e.out > junit.xml' EXIT

go test -v -timeout 15m ./test/e2e | tee test-e2e.out
