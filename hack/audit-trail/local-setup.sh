#!/usr/bin/env bash
# Spins up a local Kubernetes cluster running Argo CD built from this branch, with the audit trail enabled
# (argocd-server -> argocd-audit-controller), plus two demo users:
#   alice  - may get and sync applications in the default project
#   bob    - read only
#
# Usage:
#   hack/audit-trail/local-setup.sh            # minikube (default)
#   CLUSTER=kind hack/audit-trail/local-setup.sh
#
# Needs: the docker CLI, kubectl, jq, openssl, and minikube or kind. kind also needs a running Docker daemon;
# minikube does not (it falls back to a VirtualBox VM with its own daemon).
# Re-running the script rebuilds the image and re-deploys; `hack/audit-trail/local-setup.sh down` deletes the cluster.
set -euo pipefail

CLUSTER="${CLUSTER:-minikube}"          # minikube | kind
PROFILE="${PROFILE:-argocd-audit}"      # minikube profile / kind cluster name
IMAGE="${IMAGE:-argocd-audit:dev}"
NAMESPACE=argocd
SRCROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WORKDIR="${SRCROOT}/dist/audit-trail"

log() { printf '\n\033[1;34m==> %s\033[0m\n' "$*"; }
need() { command -v "$1" >/dev/null 2>&1 || { echo "Missing required tool: $1" >&2; exit 1; }; }

if [ "${1:-}" = "down" ]; then
  if [ "$CLUSTER" = kind ]; then kind delete cluster --name "$PROFILE"; else minikube delete -p "$PROFILE"; fi
  exit 0
fi

for t in docker kubectl jq openssl "$CLUSTER"; do need "$t"; done

log "Creating the $CLUSTER cluster '$PROFILE' (if it does not exist yet)"
if [ "$CLUSTER" = kind ]; then
  kind get clusters | grep -qx "$PROFILE" || kind create cluster --name "$PROFILE"
  kubectl config use-context "kind-$PROFILE"
else
  # minikube runs its own Docker daemon, which the image is built with, so a local Docker daemon is optional:
  # without one, minikube runs in a VirtualBox VM (set MINIKUBE_DRIVER to use another driver).
  DRIVER="${MINIKUBE_DRIVER:-}"
  if [ -z "$DRIVER" ] && ! docker info >/dev/null 2>&1; then DRIVER=virtualbox; fi
  minikube status -p "$PROFILE" >/dev/null 2>&1 || \
    minikube start -p "$PROFILE" --cpus="${MINIKUBE_CPUS:-4}" --memory="${MINIKUBE_MEMORY:-8g}" --disk-size=40g ${DRIVER:+--driver="$DRIVER"}
  kubectl config use-context "$PROFILE"
fi

log "Building the Argo CD image $IMAGE from this branch (the first build takes a while: it builds the UI too)"
if [ "$CLUSTER" = kind ]; then
  DOCKER_BUILDKIT=1 docker build -t "$IMAGE" "$SRCROOT"
  kind load docker-image "$IMAGE" --name "$PROFILE"
else
  # Build straight into minikube's Docker daemon: no registry or image loading needed.
  (eval "$(minikube -p "$PROFILE" docker-env)" && DOCKER_BUILDKIT=1 docker build -t "$IMAGE" "$SRCROOT")
fi

log "Installing Argo CD with the audit controller"
kubectl get ns "$NAMESPACE" >/dev/null 2>&1 || kubectl create ns "$NAMESPACE"
kubectl -n "$NAMESPACE" get secret argocd-audit-token >/dev/null 2>&1 || \
  kubectl -n "$NAMESPACE" create secret generic argocd-audit-token --from-literal=token="$(openssl rand -hex 32)"

rm -rf "$WORKDIR" && mkdir -p "$WORKDIR"
cat > "$WORKDIR/kustomization.yaml" <<EOF
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: ${NAMESPACE}
resources:
  - ../../manifests/cluster-install-with-audit
images:
  - name: quay.io/argoproj/argocd
    newName: ${IMAGE%%:*}
    newTag: ${IMAGE##*:}
patches:
  # Demo users and RBAC.
  - target: {kind: ConfigMap, name: argocd-cm}
    patch: |-
      - op: add
        path: /data
        value:
          accounts.alice: login
          accounts.bob: login
  - target: {kind: ConfigMap, name: argocd-rbac-cm}
    patch: |-
      - op: add
        path: /data
        value:
          policy.csv: |
            p, role:deployer, applications, get, default/*, allow
            p, role:deployer, applications, sync, default/*, allow
            g, alice, role:deployer
            g, bob, role:readonly
EOF
# The image only exists inside the cluster, so it must not be pulled.
kubectl kustomize "$WORKDIR" | sed 's/imagePullPolicy: Always/imagePullPolicy: IfNotPresent/' > "$WORKDIR/install.yaml"
kubectl apply -n "$NAMESPACE" --server-side --force-conflicts -f "$WORKDIR/install.yaml" >/dev/null
# Pick up a rebuilt image with the same tag.
kubectl -n "$NAMESPACE" rollout restart deployment >/dev/null
kubectl -n "$NAMESPACE" rollout restart statefulset >/dev/null

log "Waiting for Argo CD to be ready"
kubectl -n "$NAMESPACE" rollout status deploy/argocd-audit-controller --timeout=300s
kubectl -n "$NAMESPACE" rollout status deploy/argocd-server --timeout=300s
kubectl -n "$NAMESPACE" rollout status deploy/argocd-repo-server --timeout=300s
kubectl -n "$NAMESPACE" rollout status statefulset/argocd-application-controller --timeout=300s

ADMIN_PASSWORD="$(kubectl -n "$NAMESPACE" get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 --decode)"
cat <<EOF

Argo CD is running with the audit trail enabled.

  UI / API:        kubectl -n ${NAMESPACE} port-forward svc/argocd-server 8080:443
                   then open https://localhost:8080  (admin / ${ADMIN_PASSWORD})
  Demo scenario:   hack/audit-trail/demo.sh   (sets passwords for alice and bob, then does things as each user)
  Audit trail:     kubectl -n ${NAMESPACE} logs -f deploy/argocd-audit-controller
  Tear down:       CLUSTER=${CLUSTER} hack/audit-trail/local-setup.sh down
EOF
