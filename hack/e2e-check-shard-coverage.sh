#!/bin/bash
# Fail if a top-level e2e test did not run in any shard.
#
# Usage: hack/e2e-check-shard-coverage.sh <shard-count> <junit.xml>...
#
# Pass the JUnit report of every shard from one Kubernetes version, e.g. the
# e2e-test-timings-shard* artifacts of a CI run. The tests that should have run are read
# from the source: listing them with `go test -list` needs a running e2e environment,
# which the job that calls this does not have. Subtests are ignored, since a test and
# all its subtests always run together.
#
# Skipped tests count as run, because gotestsum still writes them to the report. A test
# that no shard selected is absent from every report, and that is what this catches.
set -eu -o pipefail

if [ "$#" -lt 2 ]; then
	echo "usage: $0 <shard-count> <junit.xml>..." >&2
	exit 2
fi

count="$1"
shift

if ! [[ "$count" =~ ^[0-9]+$ ]] || [ "$count" -lt 1 ]; then
	echo "e2e-check-shard-coverage: invalid shard count '$count'" >&2
	exit 2
fi

if [ "$#" -ne "$count" ]; then
	echo "e2e-check-shard-coverage: got $# reports but expected $count, one per shard" >&2
	exit 1
fi

dir="${E2E_TEST_DIR:-test/e2e}"

expected=$(grep -hE '^func Test[A-Za-z0-9_]*\([A-Za-z_][A-Za-z0-9_]* \*testing\.T\)' "$dir"/*_test.go \
	| sed -E 's/^func (Test[A-Za-z0-9_]*)\(.*/\1/' | sort -u)
if [ -z "$expected" ]; then
	echo "e2e-check-shard-coverage: no tests found in $dir" >&2
	exit 1
fi

ran=$(grep -hoE '<testcase [^>]*name="Test[A-Za-z0-9_]*"' "$@" \
	| sed -E 's/.*name="([^"]*)"/\1/' | sort -u)

missing=$(comm -23 <(echo "$expected") <(echo "$ran"))
if [ -n "$missing" ]; then
	echo "e2e-check-shard-coverage: these tests did not run in any of the $count shards:" >&2
	echo "$missing" >&2
	exit 1
fi

echo "all $(echo "$expected" | wc -l | tr -d ' ') e2e tests ran in the $count shards"
