#!/bin/bash
# Regenerate test/e2e/shard-weights.txt from gotestsum JUnit reports.
#
# Usage: hack/e2e-shard-weights.sh <junit.xml>... > test/e2e/shard-weights.txt
#
# Pass one report per shard from a green run, e.g. after downloading the
# e2e-test-timings-shard* artifacts:
#
#   gh run download <run-id> -p 'e2e-test-timings-shard*'
#   ./hack/e2e-shard-weights.sh e2e-test-timings-shard*/junit.xml > test/e2e/shard-weights.txt
#
# Subtest entries are ignored: gotestsum emits them before their parent and the parent's
# own time already includes them, so counting both double-counts. When gotestsum has
# retried a test (--rerun-fails), each attempt appears as its own element and the last one
# is kept -- that is the retained attempt, and it keeps weights stable across runs instead
# of letting a flake's rerun cost distort the packing.
set -eu -o pipefail

if [ "$#" -eq 0 ]; then
	echo "usage: $0 <junit.xml>..." >&2
	exit 1
fi

echo "# Per-test e2e durations in seconds, consumed by hack/e2e-shard.sh."
echo "# Regenerate with hack/e2e-shard-weights.sh; see that script for the command."
echo "# Tests absent here are given the mean weight, so a stale entry is not fatal."

awk '
	{
		line = $0
		while (match(line, /<testcase [^>]*>/)) {
			tc = substr(line, RSTART, RLENGTH)
			line = substr(line, RSTART + RLENGTH)
			name = ""; secs = 0
			# Anchor on the leading space: an unanchored /name="/ matches inside
			# classname="..." first, since awk takes the leftmost match.
			if (match(tc, /[[:space:]]name="[^"]*"/)) name = substr(tc, RSTART + 7, RLENGTH - 8)
			if (match(tc, /[[:space:]]time="[^"]*"/)) secs = substr(tc, RSTART + 7, RLENGTH - 8) + 0
			if (name == "") continue
			if (name !~ /^Test[A-Za-z0-9_]*$/) continue
			total[name] = secs
		}
	}
	END { for (n in total) printf "%s %.3f\n", n, total[n] }
' "$@" | sort
