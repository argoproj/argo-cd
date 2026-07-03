#!/usr/bin/env bash

if [ "$1" == "" ] || [ "$2" == "" ] || [ "$3" == "" ]; then
cat <<-EOM
USAGE:

  generate-release-notes.sh NEW_REF OLD_REF NEW_VERSION

EXAMPLES:

  # For releasing a new minor version:
  generate-release-notes.sh release-2.5 release-2.4 v2.5.0-rc1 > /tmp/release.md

  # For a patch release:
  generate-release-notes.sh release-2.4 v2.4.13 v2.4.14 > /tmp/release.md
EOM
exit 1
fi

function to_list_items() {
  sed 's/^/- /'
}

function strip_last_word() {
  sed 's/ [^ ]*$//'
}

function nonempty_line_count() {
  sed '/^\s*$/d' | wc -l | tr -d ' \n'
}

function only_last_word() {
  awk 'NF>1{print $NF}'
}

new_ref=$1
old_ref=$2
version=$3

cat <<-EOM
## Quick Start

### Non-HA:

\`\`\`shell
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/$version/manifests/install.yaml
\`\`\`

### HA:

\`\`\`shell
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/$version/manifests/ha/install.yaml
\`\`\`

## Release signatures

All Argo CD container images and CLI binaries are signed by cosign. See the [documentation](https://argo-cd.readthedocs.io/en/stable/operator-manual/signed-release-assets/) on how to verify the signatures.
\`\`\`shell
-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEesHEB7vX5Y2RxXypjMy1nI1z7iRG
JI9/gt/sYqzpsa65aaNP4npM43DDxoIy/MQBo9s/mxGxmA+8UXeDpVC9vw==
-----END PUBLIC KEY-----
\`\`\`

## Upgrading

If upgrading from a different minor version, be sure to read the [upgrading](https://argo-cd.readthedocs.io/en/stable/operator-manual/upgrading/overview/) documentation.

EOM

# Adapted from https://stackoverflow.com/a/67029088/684776
less_log=$(git log --pretty="format:%s %ae" --cherry-pick --left-only --no-merges "$new_ref...$old_ref")
more_log=$(git log --pretty="format:%s %ae" "$new_ref..$old_ref")

new_commits=$(diff --new-line-format="" --unchanged-line-format="" <(echo "$less_log") <(echo "$more_log") | grep -v "Merge pull request from GHSA")
new_commits_no_email=$(echo "$new_commits" | strip_last_word)
features=$(echo "$new_commits_no_email" | grep '^feat' | to_list_items)
fixes=$(echo "$new_commits_no_email" | grep '^fix' | to_list_items)
docs=$(echo "$new_commits_no_email" | grep '^docs' | to_list_items)
other=$(echo "$new_commits_no_email" | grep -v -e '^feat' -e '^fix' -e '^docs' -e '^\[Bot\]' | to_list_items)

contributors_num=$(echo "$new_commits" | only_last_word | sort -u | nonempty_line_count)

new_commits_num=$(echo "$new_commits" | nonempty_line_count)
features_num=$(echo "$features" | nonempty_line_count)
fixes_num=$(echo "$fixes" | nonempty_line_count)
docs_num=$(echo "$docs" | nonempty_line_count)
other_num=$(echo "$other" | nonempty_line_count)

previous_contributors=$(git log --pretty="format:%an %ae" "$old_ref" | sort -uf)
all_contributors=$(git log --pretty="format:%an %ae" "$new_ref" | sort -uf)
new_contributors=$(diff --new-line-format="" --unchanged-line-format="" <(echo "$all_contributors") <(echo "$previous_contributors"))
new_contributors_num=$(echo "$new_contributors" | only_last_word | nonempty_line_count)  # Count contributors by email
new_contributors_names=$(echo "$new_contributors" | strip_last_word | to_list_items)

new_contributors_message=""
if [ "$new_contributors_num" -gt 0 ]; then
  new_contributors_message=" ($new_contributors_num of them new)"
fi

echo "## Changes"
echo
echo "This release includes $new_commits_num contributions from $contributors_num contributors$new_contributors_message with $features_num features and $fixes_num bug fixes."
echo
if [ "$new_contributors_num" -lt 20 ] && [ "$new_contributors_num" -gt 0 ]; then
  echo "A special thanks goes to the $new_contributors_num new contributors:"
  echo "$new_contributors_names"
  echo
fi
if [ "$features_num" -gt 0 ]; then
  echo "### Features ($features_num)"
  echo
  echo "$features"
  echo
fi
if [ "$fixes_num" -gt 0 ]; then
  echo "### Bug fixes ($fixes_num)"
  echo
  echo "$fixes"
  echo
fi
if [ "$docs_num" -gt 0 ]; then
  echo "### Documentation ($docs_num)"
  echo
  echo "$docs"
  echo
fi
if [ "$other_num" -gt 0 ]; then
  echo "### Other ($other_num)"
  echo
  echo "$other"
  echo
fi
