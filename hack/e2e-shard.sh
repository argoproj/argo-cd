#!/bin/bash
# Print a `go test -run` regex selecting one shard of the top-level tests in a package.
#
# Usage: hack/e2e-shard.sh <package> <shard> <shard-count>   # <shard> is 1-based
#
# Tests are packed by measured duration rather than by position: heaviest first, each
# one going to the lightest shard so far (longest-processing-time bin packing). Position
# based splitting leaves stragglers, because the slow tests cluster together in the same
# file and so sit next to each other in `go test -list` order.
#
# Durations come from test/e2e/shard-weights.txt, regenerated from CI artifacts by
# hack/e2e-shard-weights.sh. Tests missing from that file, including every test when the
# file does not exist yet, get the mean weight, so the split stays sane between refreshes.
#
# Requires a running e2e environment. `go test -list` links and starts the test binary,
# so test/e2e/fixture's init() runs: it builds clients from the kubeconfig with
# NewForConfigOrDie and dials the API server via grpcutil.TestTLS. With no reachable
# server this blocks for the 30s dial timeout and then exits rather than printing names.
# In CI that is satisfied because the suite runs after `make start-e2e-local`.
#
# Only top-level test names are matched. `go test` splits a -run pattern on "/" and
# applies each element to one nesting level, so a pattern with no "/" filters top-level
# tests only and every subtest of a selected test still runs.
set -eu -o pipefail

pkg="$1"
shard="$2"
count="$3"
weights="${ARGOCD_E2E_SHARD_WEIGHTS:-test/e2e/shard-weights.txt}"

if ! [[ "$shard" =~ ^[0-9]+$ ]] || ! [[ "$count" =~ ^[0-9]+$ ]] || [ "$count" -lt 1 ] || [ "$shard" -lt 1 ] || [ "$shard" -gt "$count" ]; then
	echo "e2e-shard: invalid shard '$shard' of '$count' (expected 1 <= shard <= count)" >&2
	exit 1
fi

# `go test -list` emits the matching names followed by a trailing "ok <pkg> <time>" line.
names=$(go test -list '.*' "$pkg" | grep -E '^Test[A-Za-z0-9_]*$')

if [ -z "$names" ]; then
	echo "e2e-shard: no tests found in $pkg" >&2
	exit 1
fi

# Emit "<weight>\t<name>", heaviest first, then greedily fill the lightest shard.
# The sort makes the packing deterministic: equal weights break ties on name, and the
# scan for the lightest shard keeps the lowest index on ties.
selected=$(echo "$names" | awk -v wf="$weights" '
	BEGIN {
		known = 0; total = 0
		while ((getline line < wf) > 0) {
			if (line ~ /^[[:space:]]*(#|$)/) continue
			split(line, f, /[[:space:]]+/)
			if (f[1] == "") continue
			w[f[1]] = f[2] + 0
			total += f[2] + 0
			known++
		}
		close(wf)
		mean = (known > 0) ? total / known : 1
	}
	{ printf "%.3f\t%s\n", (($0 in w) ? w[$0] : mean), $0 }
' | sort -k1,1nr -k2,2 | awk -F'\t' -v shard="$shard" -v count="$count" '
	{
		best = 1
		for (i = 2; i <= count; i++) if (load[i] < load[best]) best = i
		load[best] += $1
		if (best == shard) print $2
	}
')

if [ -z "$selected" ]; then
	echo "e2e-shard: shard $shard of $count selected no tests in $pkg" >&2
	exit 1
fi

# Anchored alternation, e.g. ^(TestFoo|TestBar)$. Test names never contain whitespace,
# so the result is safe to pass through an unquoted TEST_FLAGS.
echo "^($(echo "$selected" | paste -sd'|' -))\$"
