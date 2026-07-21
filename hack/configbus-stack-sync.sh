#!/usr/bin/env bash
# Sync the local configbus branch stack to a GitHub fork and (optionally) open
# stacked PRs once.
#
# Typical loop while reworking:
#   ./hack/configbus-stack-sync.sh push
#
# First time (after a successful push), open the stacked PRs on the fork:
#   ./hack/configbus-stack-sync.sh open-prs
#
# Env overrides (push destination is hard-locked to crenshaw-dev/argo-cd):
#   FORK_REMOTE=crenshaw-dev       git remote name for the fork (URL must match lock)
#   UPSTREAM_REMOTE=origin         fetch-only remote for argoproj/argo-cd
#   UPSTREAM_BRANCH=master         branch mirrored onto the fork as master
#   FORK_BASE_BRANCH=master        base branch on the fork (receives upstream)
#   FORCE_PUSH=1                   1 = git push --force (default); 0 = --force-with-lease
#   DRAFT=1                        1 = open PRs as drafts (default)
#
# Safety: this script NEVER pushes to origin / argoproj. Push remotes are validated
# against ALLOWED_FORK_* below before every git push / gh pr create.
set -euo pipefail

# Hard lock — do not widen without intentionally editing this file.
ALLOWED_FORK_OWNER="crenshaw-dev"
ALLOWED_FORK_REPO="argo-cd"
ALLOWED_FORK_SLUG="${ALLOWED_FORK_OWNER}/${ALLOWED_FORK_REPO}"

FORK_REMOTE="${FORK_REMOTE:-crenshaw-dev}"
UPSTREAM_REMOTE="${UPSTREAM_REMOTE:-origin}"
UPSTREAM_BRANCH="${UPSTREAM_BRANCH:-master}"
FORK_BASE_BRANCH="${FORK_BASE_BRANCH:-master}"
FORCE_PUSH="${FORCE_PUSH:-1}"
DRAFT="${DRAFT:-1}"

# Ordered stack: each entry is head of that layer's PR.
# Index 0 bases on FORK_BASE_BRANCH; later layers base on the previous branch.
# Foundation lives in 01-controller (Provider + first consumer) for reviewability.
STACK="
configbus/01-controller
configbus/02-server
configbus/03-reposerver
configbus/04-appset
configbus/05-notifications
configbus/06-commitserver
configbus/07-shared-keys-and-coverage-metric
configbus/08-crd
configbus/09-crd-only
"

usage() {
  cat <<'EOF'
Usage: hack/configbus-stack-sync.sh <command>

Commands:
  push       Fetch upstream, force-push fork base (upstream master) + full stack
  open-prs   Create missing stacked PRs on the fork (idempotent)
  sync       push, then open-prs (convenience for first setup)
  status     Show local tips and open fork PRs for the stack
  print      Print the stack and inferred fork owner/repo

EOF
}

die() { echo "error: $*" >&2; exit 1; }

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

stack_branches() {
  # shellcheck disable=SC2086
  echo ${STACK}
}

pr_title_for() {
  case "$1" in
    configbus/01-controller) echo "feat: add configbus Provider and wire application-controller" ;;
    configbus/02-server) echo "feat: wire argocd-server through configbus" ;;
    configbus/03-reposerver) echo "feat: wire argocd-repo-server through configbus" ;;
    configbus/04-appset) echo "feat: wire applicationset-controller through configbus" ;;
    configbus/05-notifications) echo "feat: wire notifications-controller through configbus" ;;
    configbus/06-commitserver) echo "feat: wire commitserver through configbus" ;;
    configbus/07-shared-keys-and-coverage-metric) echo "feat: shared configbus keys and resolver coverage" ;;
    configbus/08-crd) echo "feat: add ArgoCDConfiguration CRD and configbus CRD source" ;;
    configbus/09-crd-only) echo "feat: CRD-only cutover for mapped configbus settings" ;;
    *) echo "feat: $1" ;;
  esac
}

force_flag() {
  if [ "${FORCE_PUSH}" = "1" ]; then
    echo "--force"
  else
    echo "--force-with-lease"
  fi
}

# Normalize a github remote URL to "owner/repo" (lowercase owner/repo path).
remote_slug_from_url() {
  local url="$1"
  local path
  path="$(echo "${url}" | sed -E 's#^ssh://##; s#^git@# #; s#^https?://##; s#^[^/]*github\.com[:/]##' | sed -E 's#\.git$##; s#/$##')"
  # path should now be owner/repo[...]; take first two segments
  echo "${path}" | awk -F/ '{print tolower($1) "/" tolower($2)}'
}

remote_url() {
  git remote get-url "$1"
}

remote_slug() {
  remote_slug_from_url "$(remote_url "$1")"
}

is_blocked_upstream_slug() {
  case "$1" in
    argoproj/argo-cd|argoproj/argo-cd.*) return 0 ;;
  esac
  return 1
}

# Abort loudly if anything about the push destination looks like upstream/origin.
assert_fork_push_safe() {
  require_cmd git

  if [ "${FORK_REMOTE}" = "origin" ]; then
    die "refusing to push: FORK_REMOTE is 'origin' (upstream). Use remote '${ALLOWED_FORK_OWNER}'."
  fi
  if [ "${FORK_REMOTE}" = "${UPSTREAM_REMOTE}" ]; then
    die "refusing to push: FORK_REMOTE (${FORK_REMOTE}) equals UPSTREAM_REMOTE — fork and upstream must differ"
  fi

  git remote get-url "${FORK_REMOTE}" >/dev/null 2>&1 \
    || die "fork remote '${FORK_REMOTE}' does not exist"

  local fork_url fork_slug upstream_url upstream_slug
  fork_url="$(remote_url "${FORK_REMOTE}")"
  fork_slug="$(remote_slug_from_url "${fork_url}")"

  if is_blocked_upstream_slug "${fork_slug}"; then
    die "refusing to push: remote '${FORK_REMOTE}' points at upstream ${fork_slug} (${fork_url})"
  fi
  if [ "${fork_slug}" != "${ALLOWED_FORK_SLUG}" ]; then
    die "refusing to push: remote '${FORK_REMOTE}' is ${fork_slug}, allowed only ${ALLOWED_FORK_SLUG} (${fork_url})"
  fi

  # Belt-and-suspenders string checks on the raw URL.
  case "${fork_url}" in
    *github.com[/:]${ALLOWED_FORK_OWNER}/${ALLOWED_FORK_REPO}*)
      ;;
    *)
      die "refusing to push: remote '${FORK_REMOTE}' URL does not contain github.com/${ALLOWED_FORK_SLUG}: ${fork_url}"
      ;;
  esac
  case "${fork_url}" in
    *github.com[/:]argoproj/argo-cd*|*github.com[/:]argoproj/argo-cd.git*)
      die "refusing to push: remote '${FORK_REMOTE}' URL looks like argoproj upstream: ${fork_url}"
      ;;
  esac

  git remote get-url "${UPSTREAM_REMOTE}" >/dev/null 2>&1 \
    || die "upstream remote '${UPSTREAM_REMOTE}' does not exist"
  upstream_url="$(remote_url "${UPSTREAM_REMOTE}")"
  upstream_slug="$(remote_slug_from_url "${upstream_url}")"
  if [ "${upstream_slug}" = "${fork_slug}" ]; then
    die "refusing to push: upstream remote '${UPSTREAM_REMOTE}' and fork remote '${FORK_REMOTE}' resolve to the same slug ${fork_slug}"
  fi
  if ! is_blocked_upstream_slug "${upstream_slug}"; then
    # Soft warning only if upstream isn't argoproj — still OK to fetch, but call it out.
    echo "warning: UPSTREAM_REMOTE '${UPSTREAM_REMOTE}' is ${upstream_slug}, expected argoproj/argo-cd" >&2
  fi

  echo "==> push safety OK: remote=${FORK_REMOTE} url=${fork_url} slug=${fork_slug}"
}

fork_slug() {
  echo "${ALLOWED_FORK_SLUG}"
}

# Only push path in this script. remote arg must be FORK_REMOTE and pass assert.
safe_git_push_to_fork() {
  local remote="$1"
  shift

  assert_fork_push_safe

  if [ "${remote}" != "${FORK_REMOTE}" ]; then
    die "refusing to push: internal remote arg '${remote}' != FORK_REMOTE '${FORK_REMOTE}'"
  fi
  if [ "${remote}" = "origin" ] || [ "${remote}" = "${UPSTREAM_REMOTE}" ]; then
    die "refusing to push: remote '${remote}' is upstream/origin"
  fi

  local slug
  slug="$(remote_slug "${remote}")"
  if [ "${slug}" != "${ALLOWED_FORK_SLUG}" ]; then
    die "refusing to push: ${remote} resolved to ${slug}, not ${ALLOWED_FORK_SLUG}"
  fi

  echo "==> git push $(force_flag) ${remote} $*"
  git push "$(force_flag)" "${remote}" "$@"
}

ensure_local_stack() {
  local b
  for b in $(stack_branches); do
    git show-ref --verify --quiet "refs/heads/${b}" || die "missing local branch ${b}"
  done
}

cmd_print() {
  assert_fork_push_safe
  echo "upstream:  ${UPSTREAM_REMOTE}/${UPSTREAM_BRANCH} ($(remote_slug "${UPSTREAM_REMOTE}"))"
  echo "fork:      $(fork_slug)  (remote ${FORK_REMOTE} -> $(remote_url "${FORK_REMOTE}"), base ${FORK_BASE_BRANCH})"
  echo "force:     $(force_flag)"
  echo "stack:"
  local i=0
  local base="${FORK_BASE_BRANCH}"
  local b
  for b in $(stack_branches); do
    printf '  %2d  %-45s  PR base=%s\n' "${i}" "${b}" "${base}"
    base="${b}"
    i=$((i + 1))
  done
}

cmd_status_brief() {
  local b
  echo "local tips:"
  for b in $(stack_branches); do
    printf '  %s  %s\n' "$(git rev-parse --short "${b}")" "${b}"
  done
}

cmd_push() {
  require_cmd git
  ensure_local_stack
  assert_fork_push_safe

  echo "==> fetching ${UPSTREAM_REMOTE} (read-only; never pushed)"
  git fetch --prune "${UPSTREAM_REMOTE}" "${UPSTREAM_BRANCH}"

  echo "==> push ${UPSTREAM_REMOTE}/${UPSTREAM_BRANCH} -> ${FORK_REMOTE}:${FORK_BASE_BRANCH}"
  safe_git_push_to_fork "${FORK_REMOTE}" \
    "refs/remotes/${UPSTREAM_REMOTE}/${UPSTREAM_BRANCH}:refs/heads/${FORK_BASE_BRANCH}"

  echo "==> push stack -> ${FORK_REMOTE}"
  # One push keeps refs consistent on the fork.
  # shellcheck disable=SC2046
  safe_git_push_to_fork "${FORK_REMOTE}" $(stack_branches)

  echo "==> done"
  cmd_status_brief
}

pr_exists_for_head() {
  local head="$1"
  local n
  n="$(gh pr list --repo "$(fork_slug)" --head "${head}" --state all --json number --jq 'length')"
  [ "${n}" != "0" ]
}

cmd_open_prs() {
  require_cmd gh
  require_cmd git
  ensure_local_stack
  assert_fork_push_safe

  local slug
  slug="$(fork_slug)"
  if [ "${slug}" != "${ALLOWED_FORK_SLUG}" ]; then
    die "refusing open-prs: slug ${slug} != ${ALLOWED_FORK_SLUG}"
  fi
  if [ "${slug}" = "argoproj/argo-cd" ]; then
    die "refusing open-prs: would target upstream argoproj/argo-cd"
  fi

  echo "==> opening stacked PRs on ${slug} (skip existing)"

  local base="${FORK_BASE_BRANCH}"
  local head title body
  local draft_args=""
  if [ "${DRAFT}" = "1" ]; then
    draft_args="--draft"
  fi

  for head in $(stack_branches); do
    if pr_exists_for_head "${head}"; then
      echo "  skip  ${head} (PR already exists)"
      base="${head}"
      continue
    fi

    title="$(pr_title_for "${head}")"
    body="$(cat <<EOF
## Summary
Stacked configbus PR.

- **Head:** \`${head}\`
- **Base:** \`${base}\`

Part of the local configbus migration stack. Rework with \`./hack/configbus-stack-sync.sh push\`.

## Test plan
- [ ] \`go test ./util/configbus/\`
- [ ] Review layer-specific wiring / docs for this branch only
EOF
)"

    echo "  create ${head} -> ${base} on ${slug}"
    # shellcheck disable=SC2086
    gh pr create \
      --repo "${slug}" \
      --base "${base}" \
      --head "${head}" \
      --title "${title}" \
      --body "${body}" \
      ${draft_args}

    base="${head}"
  done

  echo "==> PR list"
  gh pr list --repo "${slug}" --search "head:configbus/" --limit 20
}

cmd_status() {
  require_cmd git
  require_cmd gh
  ensure_local_stack
  assert_fork_push_safe

  local slug
  slug="$(fork_slug)"
  echo "fork: ${slug} (remote ${FORK_REMOTE} -> $(remote_url "${FORK_REMOTE}"))"
  echo
  cmd_status_brief
  echo
  echo "open PRs on ${slug}:"
  gh pr list --repo "${slug}" --search "head:configbus/" --limit 20 || true
}

cmd_sync() {
  cmd_push
  cmd_open_prs
}

main() {
  local cmd="${1:-}"
  case "${cmd}" in
    push) cmd_push ;;
    open-prs|open_prs) cmd_open_prs ;;
    sync) cmd_sync ;;
    status) cmd_status ;;
    print) cmd_print ;;
    -h|--help|help) usage ;;
    "") usage; exit 1 ;;
    *) die "unknown command: ${cmd} (try --help)" ;;
  esac
}

main "$@"
