#!/usr/bin/env bash
# Exercises the audit trail on a cluster set up by hack/audit-trail/local-setup.sh: several users do allowed and
# forbidden things through the Argo CD API, then the audit trail is printed as a table.
#
# Needs: kubectl, jq, curl.
set -uo pipefail

NAMESPACE=argocd
PORT="${PORT:-8080}"
# ARGOCD_URL and AUDIT_LOG_CMD point the demo at an Argo CD that is not port-forwarded from the cluster
# (e.g. one started with `make start-local`).
ARGOCD_URL="${ARGOCD_URL:-}"
AUDIT_LOG_CMD="${AUDIT_LOG_CMD:-kubectl -n $NAMESPACE logs deploy/argocd-audit-controller}"
J='Content-Type: application/json'

if [ -z "$ARGOCD_URL" ]; then
  ARGOCD_URL="https://localhost:${PORT}"
  kubectl -n "$NAMESPACE" port-forward svc/argocd-server "${PORT}:443" >/dev/null 2>&1 &
  PF=$!
  trap 'kill $PF 2>/dev/null' EXIT
  sleep 3
fi
API="${ARGOCD_URL}/api/v1"

ADMIN_PASSWORD="$(kubectl -n "$NAMESPACE" get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 --decode)"

login() { curl -sk -H "$J" "$API/session" -d "{\"username\":\"$1\",\"password\":\"$2\"}" | jq -r '.token // empty'; }
call() { # user-token method path [body]
  local code
  code=$(curl -sk -o /dev/null -w '%{http_code}' -H "$J" -H "Authorization: Bearer $1" -X "$2" "$API$3" ${4:+-d "$4"})
  printf '    %-7s %-60s -> HTTP %s\n' "$2" "$3" "$code"
}
step() { printf '\n\033[1m%s\033[0m\n' "$*"; }

step "mallory tries to log in with a guessed password"
login mallory guess >/dev/null

step "admin logs in, sets alice's and bob's passwords and creates the guestbook app"
ADMIN=$(login admin "$ADMIN_PASSWORD")
call "$ADMIN" PUT /account/password "{\"name\":\"alice\",\"currentPassword\":\"$ADMIN_PASSWORD\",\"newPassword\":\"alice-password\"}"
call "$ADMIN" PUT /account/password "{\"name\":\"bob\",\"currentPassword\":\"$ADMIN_PASSWORD\",\"newPassword\":\"bob-password\"}"
call "$ADMIN" POST "/applications?upsert=true" '{
  "metadata": {"name": "guestbook"},
  "spec": {
    "project": "default",
    "source": {"repoURL": "https://github.com/argoproj/argocd-example-apps.git", "path": "guestbook", "targetRevision": "HEAD"},
    "destination": {"server": "https://kubernetes.default.svc", "namespace": "default"}
  }}'

step "alice syncs guestbook (allowed), then tries to delete it and a cluster (denied)"
ALICE=$(login alice alice-password)
call "$ALICE" POST /applications/guestbook/sync '{"prune": true}'
call "$ALICE" DELETE /applications/guestbook
call "$ALICE" DELETE "/clusters/https%3A%2F%2Fkubernetes.default.svc"

step "bob (read only) tries to sync"
BOB=$(login bob bob-password)
call "$BOB" POST /applications/guestbook/sync '{}'

step "a request with a forged token"
call "not-a-token" DELETE /applications/guestbook

step "admin turns on automated sync: the application controller now syncs guestbook by itself"
call "$ADMIN" PATCH /applications/guestbook '{"name":"guestbook","patchType":"merge","patch":"{\"spec\":{\"syncPolicy\":{\"automated\":{\"prune\":true}}}}"}'

sleep 15
step "The audit trail ($AUDIT_LOG_CMD):"
printf '%-9s %-38s %-34s %-24s %-8s %s\n' TIME USER ACTION RESOURCE RESULT CODE
$AUDIT_LOG_CMD \
  | jq -r 'select(.kind == "AuditRecord") | [.timestamp[11:19], .actor.username, .action, .resource.name, .result.status, .result.code] | @tsv' \
  | while IFS=$'\t' read -r t u a r s c; do printf '%-9s %-38s %-34s %-24s %-8s %s\n' "$t" "$u" "$a" "$r" "$s" "$c"; done
