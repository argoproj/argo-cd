#!/usr/bin/env bash

set -e
set -o pipefail

npm install snyk snyk-to-html --location=global

# Choose the branch where docs changes will actually be written.
target_branch="$1"
if [ "$target_branch" != "" ]; then
  git checkout "$target_branch"
fi

# In case the directory doesn't exist in the target branch.
mkdir -p docs/snyk

# Clear the docs directory in case we need to delete an old version.
rm -rf docs/snyk/*

cat > docs/snyk/index.md <<- EOM
# Snyk Scans

Every Sunday, Snyk scans are generated for Argo CD's \`master\` branch and the most recent patches of the three most
recent minor releases.

!!! note
    For the most recent scans, view the [\`latest\` version of the docs](https://argo-cd.readthedocs.io/en/latest/snyk/).
    You can return to your preferred version of the docs site using the dropdown selector at the top of the page.

## Scans
EOM

argocd_dir=$(pwd)
temp_dir=$(mktemp -d)
cd "$temp_dir"
git clone https://github.com/argoproj/argo-cd.git
cd argo-cd
git checkout master

minor_version=$(git tag -l | sort -V | tail -n 1 | grep -Eo '[0-9]+\.[0-9]+')
patch_num=$(git tag -l | grep "v$minor_version." | grep -o "[a-z[:digit:]-]*$" | sort -V | tail -n 1)
version="v$minor_version.$patch_num"
versions="master "

version_count=3
# When the most recent version is still a release candidate, get reports for 4 versions (so the 3 most recent stable
# releases are included).
if [[ $patch_num == "0-rc"* ]]; then version_count=4; fi

for i in $(seq "$version_count"); do
  if [ "$version" == "" ]; then break; fi
  # Nightmare code to get the most recent patches of the three most recent minor versions.
  versions+="$version "
  minor_num=$(printf '%s' "$minor_version" | sed -E 's/[0-9]+\.//')
  minor_num=$((minor_num-1))
  minor_version=$(printf '%s' "$minor_version" | sed -E "s/\.[0-9]+$/.$minor_num/g")
  patch_num=$(git tag -l | grep "v$minor_version." | grep -o "[a-z[:digit:]-]*$" | sort -V | tail -n 1)
  version="v$minor_version.$patch_num"
done

for version in $versions; do
  printf '\n%s\n\n' "### $version" >> "$argocd_dir/docs/snyk/index.md"

  mkdir -p "$argocd_dir/docs/snyk/$version"

  git reset --hard # reset any pending changes to avoid checkout errors
  git checkout "$version"

  # Get the latest ignore rules.
  cp "$argocd_dir/.snyk" .snyk

  # || [ $? == 1 ] ignores errors due to vulnerabilities.
  snyk test --all-projects --exclude=docs,site,ui-test --org=argoproj --policy-path=.snyk --sarif-file-output=/tmp/argocd-test.sarif --json-file-output=/tmp/argocd-test.json || [ $? == 1 ]
  snyk-to-html -i /tmp/argocd-test.json -o "$argocd_dir/docs/snyk/$version/argocd-test.html"
  { echo "|    | Critical | High | Medium | Low |"
    echo "|---:|:--------:|:----:|:------:|:---:|"
   } >> "$argocd_dir/docs/snyk/index.md"
  jq 'map(
        {
          # Collect all the vulnerabilities severities. Group by id to avoid double-counting.
          severity: (.vulnerabilities | group_by(.id) | map(.[0])[].severity),
          displayTargetFile: (.displayTargetFile)
        }
      )
      # Hack to make sure even if there are no vulnerabilities, a row is added to the table.
      + [{displayTargetFile: "go.mod"}, {displayTargetFile: "ui/yarn.lock"}]
      # Group by target file (e.g. go.mod) so we can see where the vulnerabilities are.
      | group_by(.displayTargetFile)
      | map(
        "| [\(.[0].displayTargetFile)](\($version)/argocd-test.html) "
        + "| \(map(select(.severity == "critical")) | length) "
        + "| \(map(select(.severity == "high")) | length) "
        + "| \(map(select(.severity == "medium")) | length) "
        + "| \(map(select(.severity == "low")) | length) |")
      | join("\n")' --arg version "$version" -r /tmp/argocd-test.json >> "$argocd_dir/docs/snyk/index.md"


  images=$(grep 'image: ' manifests/install.yaml manifests/namespace-install.yaml manifests/ha/install.yaml | sed 's/.*image: //' | sort | uniq)

  while IFS= read -r image; do
    extra_args=""
    if echo "$image" | grep "argocd"; then
      # Pass the file arg only for the Argo CD image. The file arg also gives us access to sarif output.
      extra_args="--file=Dockerfile --sarif-file-output=/tmp/${image//[\/:]/_}.sarif "
    fi

    set -x
    # || [ $? == 1 ] ignores errors due to vulnerabilities.
    snyk container test "$image" --org=argoproj "--json-file-output=/tmp/${image//[\/:]/_}.json" $extra_args || [ $? == 1 ]
    set +x

    snyk-to-html -i "/tmp/${image//[\/:]/_}.json" -o "$argocd_dir/docs/snyk/$version/${image//[\/:]/_}.html"

    printf '%s' "| [${image/*\//}]($version/${image//[\/:]/_}.html) | " >> "$argocd_dir/docs/snyk/index.md"

    # Add severity counts to index.
    jq '[
      .vulnerabilities
      # Group by ID to avoid double-counting.
      | group_by(.id)
      # Get the severity of the first vulnerability in the group (should be the same for every item in the group).
      | map(.[0])[].severity
    # Construct a summary using the counts of each severity level.
    ] | "\(map(select(. == "critical")) | length) | \(map(select(. == "high")) | length) | \(map(select(. == "medium")) | length) | \(map(select(. == "low")) | length) |"
    ' -r "/tmp/${image//[\/:]/_}.json" >> "$argocd_dir/docs/snyk/index.md"
  done <<< "$images"

  # || [ $? == 1 ] ignores errors due to vulnerabilities.
  snyk iac test manifests/install.yaml --org=argoproj --policy-path=.snyk --sarif-file-output=/tmp/argocd-iac-install.sarif --json-file-output=/tmp/argocd-iac-install.json || [ $? == 1 ]
  snyk-to-html -i /tmp/argocd-iac-install.json -o "$argocd_dir/docs/snyk/$version/argocd-iac-install.html"
  echo "| [install.yaml]($version/argocd-iac-install.html) | - | - | - | - |" >> "$argocd_dir/docs/snyk/index.md"

  # || [ $? == 1 ] ignores errors due to vulnerabilities.
  snyk iac test manifests/namespace-install.yaml --org=argoproj --policy-path=.snyk --sarif-file-output=/tmp/argocd-iac-namespace-install.sarif --json-file-output=/tmp/argocd-iac-namespace-install.json || [ $? == 1 ]
  snyk-to-html -i /tmp/argocd-iac-namespace-install.json -o "$argocd_dir/docs/snyk/$version/argocd-iac-namespace-install.html"
  echo "| [namespace-install.yaml]($version/argocd-iac-namespace-install.html) | - | - | - | - |" >> "$argocd_dir/docs/snyk/index.md"
done

# clean up
cd "$argocd_dir"  # Back to the main argo-cd clone.
rm -rf "$temp_dir"

# regex-escape the temp dir path
dir_r="${temp_dir//\//\\\/}"

# Make sed -i cross-platform: https://stackoverflow.com/a/51060063/684776
sedi=(-i)
case "$(uname)" in
  Darwin*) sedi=(-i "")
esac

# remove temp dir path from Snyk output
sed "${sedi[@]}" "s/$dir_r//g" docs/snyk/*/*.html
