#!/bin/sh
set -eux

trap 'go-junit-report < test.out > junit.xml' EXIT

go test -v -covermode=count -coverprofile=coverage.out $(go list ./... | grep -v 'test/e2e') | tee test.out
