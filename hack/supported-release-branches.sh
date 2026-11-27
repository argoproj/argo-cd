#!/usr/bin/env bash
# Print currently supported release-X.Y branches, one per line, newest first.
#
# A series is supported when it is one of the three newest release-X.Y branches
# that already have a stable vX.Y.Z tag. If the newest release-X.Y branch has no
# stable tag yet, it is included too (the series is still in release candidate).
#
# Stdout is only branch names. A one-line note goes to stderr.
# Pass a git remote as the first argument; the default is origin.
set -euo pipefail

remote="${1:-origin}"

is_digits() {
  case "$1" in
    '' | *[!0-9]*) return 1 ;;
    *) return 0 ;;
  esac
}

# Assign before the loop. A process substitution would hide a failed ls-remote,
# and the script would exit 0 with an empty or partial branch list.
branch_refs="$(git ls-remote --heads "$remote" 'refs/heads/release-*' | awk '{print $2}')"
branches=()
while IFS= read -r ref; do
  name="${ref##*/}"
  case "$name" in
    release-*.*)
      version="${name#release-}"
      major="${version%%.*}"
      minor="${version#*.}"
      if is_digits "$major" && is_digits "$minor"; then
        branches+=("$name")
      fi
      ;;
  esac
done <<< "$branch_refs"

stable_lines=""
tag_refs="$(git ls-remote --tags "$remote" 'refs/tags/v*' | awk '{print $2}')"
while IFS= read -r ref; do
  name="${ref#refs/tags/}"
  peeled_suffix='^{}'
  name="${name%"$peeled_suffix"}"
  case "$name" in
    v*.*.*)
      version="${name#v}"
      major="${version%%.*}"
      rest="${version#*.}"
      minor="${rest%%.*}"
      patch="${rest#*.}"
      if is_digits "$major" && is_digits "$minor" && is_digits "$patch"; then
        stable_lines+="${major}.${minor}"$'\n'
      fi
      ;;
  esac
done <<< "$tag_refs"

stable_series="$(printf '%s' "$stable_lines" | sort -u)"

has_stable() {
  local series="$1"
  [[ -n "$stable_series" ]] && printf '%s\n' "$stable_series" | grep -qx "$series"
}

sorted=()
if ((${#branches[@]} > 0)); then
  while IFS= read -r branch; do
    sorted+=("$branch")
  done < <(
    for branch in "${branches[@]}"; do
      version="${branch#release-}"
      major="${version%%.*}"
      minor="${version#*.}"
      printf '%08d %08d %s\n' "$major" "$minor" "$branch"
    done | sort -k1,1nr -k2,2nr | awk '{print $3}'
  )
fi

selected=()
stable_count=0
newest_is_rc=0
for index in "${!sorted[@]}"; do
  branch="${sorted[$index]}"
  series="${branch#release-}"
  if has_stable "$series"; then
    if ((stable_count < 3)); then
      selected+=("$branch")
      stable_count=$((stable_count + 1))
    fi
  elif ((index == 0)); then
    newest_is_rc=1
    selected+=("$branch")
  fi
done

if ((newest_is_rc == 1)); then
  echo "Newest release series has no stable tag; including it with the ${stable_count} newest stable series." >&2
else
  echo "Including the ${stable_count} newest stable release series." >&2
fi

if ((${#selected[@]} > 0)); then
  printf '%s\n' "${selected[@]}"
fi
