#!/bin/bash
# Download the e2e test timings of the most recent successful master builds.
#
# Usage: hack/e2e-shard-weights-download.sh <out-dir> [max-runs] [days]
#
# Each run that uploaded e2e-test-timings-shard* artifacts lands in <out-dir>/<run-id>.
# Runs where the e2e jobs were skipped have no such artifacts and are passed over.
# Prints the number of runs downloaded.
set -eu -o pipefail

if [ "$#" -lt 1 ]; then
	echo "usage: $0 <out-dir> [max-runs] [days]" >&2
	exit 1
fi

out="$1"
max="${2:-10}"
days="${3:-7}"
repo="${GITHUB_REPOSITORY:-argoproj/argo-cd}"
since=$(date -u -d "-${days} days" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -v-"${days}"d +%Y-%m-%dT%H:%M:%SZ)

runs=$(gh run list -R "$repo" --workflow ci-build.yaml --branch master --event push --status success \
	--created ">=${since}" --limit 200 --json databaseId --jq '.[].databaseId')

found=0
for id in $runs; do
	[ "$found" -ge "$max" ] && break
	if gh run download -R "$repo" "$id" -p 'e2e-test-timings-shard*' -D "${out}/${id}" >/dev/null 2>&1; then
		found=$((found + 1))
	else
		rm -rf "${out:?}/${id}"
	fi
done

echo "downloaded e2e timings from ${found} runs since ${since}" >&2
echo "$found"
