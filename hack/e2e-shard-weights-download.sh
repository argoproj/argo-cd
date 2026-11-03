#!/bin/bash
# Download the e2e test timings of the most recent successful master builds.
#
# Usage: hack/e2e-shard-weights-download.sh <out-dir> <shards> [max-runs] [days]
#
# Each run that uploaded an unexpired e2e-test-timings-shard* artifact for every shard
# lands in <out-dir>/<run-id>. Other runs, such as those that skipped e2e, are passed over.
# Prints the number of runs downloaded.
set -eu -o pipefail

if [ "$#" -lt 2 ]; then
	echo "usage: $0 <out-dir> <shards> [max-runs] [days]" >&2
	exit 1
fi

out="$1"
shards="$2"
max="${3:-10}"
days="${4:-7}"
repo="${GITHUB_REPOSITORY:-argoproj/argo-cd}"
since=$(date -u -d "-${days} days" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -v-"${days}"d +%Y-%m-%dT%H:%M:%SZ)

runs=$(gh run list -R "$repo" --workflow ci-build.yaml --branch master --event push --status success \
	--created ">=${since}" --limit 200 --json databaseId --jq '.[].databaseId')

found=0
for id in $runs; do
	[ "$found" -ge "$max" ] && break
	artifacts=$(gh api "repos/${repo}/actions/runs/${id}/artifacts?per_page=100" \
		--jq '[.artifacts[] | select((.name | startswith("e2e-test-timings-shard")) and (.expired | not))] | length')
	if [ "$artifacts" -ne "$shards" ]; then
		echo "skipping run ${id}: ${artifacts} of ${shards} timing artifacts" >&2
		continue
	fi
	gh run download -R "$repo" "$id" -p 'e2e-test-timings-shard*' -D "${out}/${id}" >&2
	found=$((found + 1))
done

echo "downloaded e2e timings from ${found} runs since ${since}" >&2
echo "$found"
