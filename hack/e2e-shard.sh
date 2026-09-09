#!/bin/bash
set -eu -o pipefail

pkg="$1"
shard="$2"
count="$3"

if ! [[ "$shard" =~ ^[0-9]+$ ]] || ! [[ "$count" =~ ^[0-9]+$ ]] || [ "$count" -lt 1 ] || [ "$shard" -lt 1 ] || [ "$shard" -gt "$count" ]; then
	echo "e2e-shard: invalid shard '$shard' of '$count' (expected 1 <= shard <= count)" >&2
	exit 1
fi

names=$(go test -list '.*' "$pkg" | grep -E '^Test[A-Za-z0-9_]*$' | awk -v s="$shard" -v n="$count" 'NR % n == s % n')

if [ -z "$names" ]; then
	echo "e2e-shard: shard $shard of $count selected no tests in $pkg" >&2
	exit 1
fi

echo "^($(echo "$names" | paste -sd'|' -))\$"
