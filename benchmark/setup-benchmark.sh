#!/bin/bash
set -Eeuo pipefail

# ============================================================================
# Configuration
# ============================================================================
readonly CLUSTER_NAME="argocd-perf-test"
readonly NAMESPACE_COUNT="${NAMESPACE_COUNT:-100}"
readonly FEATURE_ENABLED="${FEATURE_ENABLED:-false}"
readonly SKIP_CLUSTER_RECREATE="${SKIP_CLUSTER_RECREATE:-true}"
readonly CONFIGURE_PROXY="${CONFIGURE_PROXY:-false}"
readonly CONFIGURE_CA_BUNDLE="${CONFIGURE_CA_BUNDLE:-false}"
readonly CLUSTER_SECRET_NAME="cluster-in-cluster"
readonly ARGOCD_NAMESPACE="argocd"

# Color codes
readonly COLOR_RED='\033[0;31m'
readonly COLOR_GREEN='\033[0;32m'
readonly COLOR_YELLOW='\033[1;33m'
readonly COLOR_RESET='\033[0m'

# ============================================================================
# Utility Functions (Rule #2: Reveal Intent)
# ============================================================================

print_header() {
  echo -e "${COLOR_GREEN}=== $1 ===${COLOR_RESET}"
}

print_step() {
  echo -e "${COLOR_YELLOW}[$1/$2] $3${COLOR_RESET}"
}

print_success() {
  echo -e "${COLOR_GREEN}✓ $1${COLOR_RESET}"
}

print_warning() {
  echo -e "${COLOR_YELLOW}⚠ $1${COLOR_RESET}"
}

print_info() {
  echo "$1"
}

# ============================================================================
# Kubernetes Helper Functions (Rule #3: No Duplication)
# ============================================================================

resource_exists() {
  kubectl get "$1" "$2" -n "$3" &>/dev/null
}

get_secret_value() {
  kubectl get secret "$1" -n "$2" -o jsonpath="$3" 2>/dev/null | base64 -d
}

wait_for_condition() {
  local resource=$1
  local condition=$2
  local timeout=${3:-300s}
  kubectl wait --for="$condition" "$resource" -n "$ARGOCD_NAMESPACE" --timeout="$timeout"
}

apply_manifest() {
  kubectl apply -f - <<EOF
$1
EOF
}

# ============================================================================
# ArgoCD Setup Functions
# ============================================================================

select_overlay() {
  local overlay_base
  if [ "$CONFIGURE_PROXY" = "true" ]; then
    overlay_base="benchmark/overlays-with-proxy"
  else
    overlay_base="benchmark/overlays"
  fi

  if [ "$FEATURE_ENABLED" = "true" ]; then
    echo "$overlay_base/feature-enabled"
  else
    echo "$overlay_base/feature-disabled"
  fi
}

select_kind_config() {
  if [ "$CONFIGURE_PROXY" = "true" ]; then
    echo "benchmark/overlays-with-proxy/argocd-kind-config.yaml"
  else
    echo "benchmark/argocd-kind-config.yaml"
  fi
}

create_kind_cluster() {
  print_step 1 7 "Creating kind cluster..."

  if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
    if [ "$SKIP_CLUSTER_RECREATE" = "true" ]; then
      print_info "Cluster $CLUSTER_NAME already exists, skipping recreation..."
      print_success "Cluster ready"
      return
    fi
    print_info "Cluster $CLUSTER_NAME already exists, deleting..."
    kind delete cluster --name "$CLUSTER_NAME"
  fi

  local kind_config
  kind_config=$(select_kind_config)
  print_info "Using kind config: $kind_config"
  kind create cluster --config "$kind_config"
  print_success "Cluster ready"
}

install_argocd() {
  print_step 2 7 "Installing ArgoCD..."

  local overlay
  overlay=$(select_overlay)
  print_info "Using overlay: $overlay"

  kubectl create namespace "$ARGOCD_NAMESPACE" 2>/dev/null || true
  kubectl apply -k "$overlay"

  print_info "Waiting for ArgoCD CRDs to be established..."
  wait_for_condition "crd/applications.argoproj.io" "condition=established" "60s"

  print_info "Creating guestbook application..."
  kubectl apply -f benchmark/base/guestbook-app.yaml

  if [ "$CONFIGURE_CA_BUNDLE" = "true" ]; then
    configure_ca_bundle
  fi
  print_success "ArgoCD installed"
}

# Corporate proxy support: Adds custom CA certificates to ArgoCD
# Only needed when behind a corporate proxy that intercepts HTTPS traffic
configure_ca_bundle() {
  local ca_bundle_file="benchmark/overlays-with-proxy/ca-bundle.crt"

  print_info "Adding corporate CA bundle to ArgoCD TLS ConfigMap..."
  if [ ! -f "$ca_bundle_file" ]; then
    print_warning "CA bundle not found at $ca_bundle_file, skipping"
    return
  fi

  local ca_content
  ca_content=$(sed 's/$/\\n/' "$ca_bundle_file" | tr -d '\n')
  kubectl patch configmap argocd-tls-certs-cm -n "$ARGOCD_NAMESPACE" \
    --type merge -p "{\"data\":{\"github.com\":\"$ca_content\"}}"

  print_info "Restarting repo-server to pick up CA certificates..."
  kubectl rollout restart deployment/argocd-repo-server -n "$ARGOCD_NAMESPACE"
}

wait_for_argocd_ready() {
  print_step 3 7 "Waiting for ArgoCD to be ready..."

  print_info "Waiting for ArgoCD pods to be created..."
  until kubectl get pods -n "$ARGOCD_NAMESPACE" 2>/dev/null | grep -q argocd; do
    sleep 2
  done

  print_info "Waiting for ArgoCD pods to be ready..."
  kubectl rollout status statefulset/argocd-application-controller -n "$ARGOCD_NAMESPACE"

  print_info "Waiting for ArgoCD server to fully initialize..."
  sleep 10

  local password
  password=$(get_secret_value "argocd-initial-admin-secret" "$ARGOCD_NAMESPACE" "{.data.password}")
  print_success "ArgoCD is ready"
  print_info "  Admin password: $password"
}

create_test_namespaces() {
  print_step 4 7 "Creating $NAMESPACE_COUNT empty test namespaces..."

  for i in $(seq 1 "$NAMESPACE_COUNT"); do
    kubectl create namespace "test-ns-$i" 2>/dev/null || true
  done

  print_success "$NAMESPACE_COUNT empty namespaces created"
}

setup_port_forward() {
  print_step 5 7 "Starting port-forward to ArgoCD server..."

  # Kill existing port-forward if running
  if lsof -ti:8080 &>/dev/null; then
    print_info "Port 8080 is in use, killing existing process..."
    kill "$(lsof -ti:8080)" 2>/dev/null || true
    sleep 1
  fi

  kubectl port-forward svc/argocd-server -n "$ARGOCD_NAMESPACE" 8080:443 &>/dev/null &
  local pid=$!
  print_info "  Port-forward PID: $pid"

  print_info "  Waiting for port-forward to be ready..."
  for i in {1..10}; do
    if curl -k https://localhost:8080 &>/dev/null; then
      break
    fi
    sleep 1
  done

  print_success "Port-forward started"
}

# ============================================================================
# Cluster Configuration Functions
# ============================================================================

build_namespace_list() {
  for i in $(seq 1 "$NAMESPACE_COUNT"); do
    echo "test-ns-$i"
  done | paste -sd ","
}

create_token_secret() {
  # Silence both stdout and stderr from kubectl apply
  kubectl apply -f - >/dev/null 2>&1 <<'EOF'
apiVersion: v1
kind: Secret
metadata:
  name: argocd-application-controller-token
  namespace: argocd
  annotations:
    kubernetes.io/service-account.name: argocd-application-controller
type: kubernetes.io/service-account-token
EOF
}

wait_for_token_creation() {
  # Wait until the token data exists, then print only the decoded token
  for i in {1..30}; do
    token=$(kubectl get secret argocd-application-controller-token -n "$ARGOCD_NAMESPACE" \
      -o jsonpath='{.data.token}' 2>/dev/null)
    if [ -n "$token" ]; then
      echo -n "$token" | base64 -d | tr -d '\n'
      return
    fi
    sleep 1
  done
  return 1
}

get_service_account_token() {
  token=$(kubectl get secret -n "$ARGOCD_NAMESPACE" \
    -o jsonpath='{.items[?(@.metadata.annotations.kubernetes\.io/service-account\.name=="argocd-application-controller")].data.token}' \
    2>/dev/null | head -n 1)

  if [ -n "$token" ]; then
    echo -n "$token" | base64 -d | tr -d '\n'
    return
  fi

  create_token_secret >/dev/null 2>&1 || true

  token_decoded="$(wait_for_token_creation)" || {
    echo "ERROR: timed out waiting for controller token" >&2
    return 1
  }
  echo -n "$token_decoded"
}

create_cluster_config() {
  local token=$1
  jq -n --arg token "$token" '{
    "bearerToken": $token,
    "tlsClientConfig": {
      "insecure": false
    }
  }'
}

create_cluster_secret() {
  local namespace_list=$1
  local token
  token=$(get_service_account_token)

  local config
  config=$(create_cluster_config "$token")
  local config_b64
  config_b64=$(echo -n "$config" | base64 -w 0)

  apply_manifest "$(cat <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: $CLUSTER_SECRET_NAME
  namespace: $ARGOCD_NAMESPACE
  labels:
    argocd.argoproj.io/secret-type: cluster
type: Opaque
data:
  name: $(echo -n "in-cluster" | base64 -w 0)
  server: $(echo -n "https://kubernetes.default.svc" | base64 -w 0)
  config: $config_b64
  namespaces: $(echo -n "$namespace_list" | base64 -w 0)
EOF
)"
}

update_cluster_namespaces() {
  local namespace_list=$1
  kubectl patch secret "$CLUSTER_SECRET_NAME" -n "$ARGOCD_NAMESPACE" \
    --type merge -p "{\"data\":{\"namespaces\":\"$(echo -n "$namespace_list" | base64 -w 0)\"}}"
}

restart_application_controller() {
  print_info "Restarting application controller to apply changes..."
  kubectl rollout restart statefulset/argocd-application-controller -n "$ARGOCD_NAMESPACE"
  kubectl rollout status statefulset/argocd-application-controller -n "$ARGOCD_NAMESPACE" --timeout=120s
}

configure_cluster_namespaces() {
  print_step 6 7 "Configuring ArgoCD cluster to watch test namespaces..."

  local namespace_list
  namespace_list=$(build_namespace_list)

  if resource_exists secret "$CLUSTER_SECRET_NAME" "$ARGOCD_NAMESPACE"; then
    print_info "In-cluster secret already exists"
    update_cluster_if_needed "$namespace_list"
  else
    print_info "Creating in-cluster secret..."
    create_cluster_secret "$namespace_list"
    print_info "In-cluster secret created successfully"
    restart_application_controller
  fi

  print_success "Cluster configured to watch $NAMESPACE_COUNT namespaces"
}

update_cluster_if_needed() {
  local expected_namespaces=$1
  local current_namespaces
  current_namespaces=$(get_secret_value "$CLUSTER_SECRET_NAME" "$ARGOCD_NAMESPACE" "{.data.namespaces}")

  if [ "$current_namespaces" = "$expected_namespaces" ]; then
    print_info "Cluster already configured with correct namespaces, skipping..."
    return
  fi

  # Check if all base namespaces (test-ns-1 to test-ns-N) are present
  # If they are, preserve any additional namespaces that were added
  local all_base_present=true
  for i in $(seq 1 "$NAMESPACE_COUNT"); do
    if ! echo ",$current_namespaces," | grep -q ",test-ns-$i,"; then
      all_base_present=false
      break
    fi
  done

  if [ "$all_base_present" = true ]; then
    local current_count
    current_count=$(echo "$current_namespaces" | awk -F',' '{print NF}')
    print_info "Cluster already contains all $NAMESPACE_COUNT base namespaces (currently has $current_count total), preserving additional namespaces..."
    return
  fi

  print_info "Updating cluster namespace configuration..."
  update_cluster_namespaces "$expected_namespaces"
  print_info "Cluster configuration updated successfully"
  restart_application_controller
}

# ============================================================================
# Application Health Check Functions
# ============================================================================

get_app_status() {
  local app_name=$1
  local status_type=$2
  kubectl get application "$app_name" -n "$ARGOCD_NAMESPACE" \
    -o jsonpath="{.status.$status_type.status}" 2>/dev/null || echo "Unknown"
}

is_app_healthy_and_synced() {
  local app_name=$1
  local health
  local sync
  health=$(get_app_status "$app_name" "health")
  sync=$(get_app_status "$app_name" "sync")

  [ "$health" = "Healthy" ] && [ "$sync" = "Synced" ]
}

trigger_app_refresh() {
  local app_name=$1
  kubectl patch application "$app_name" -n "$ARGOCD_NAMESPACE" \
    --type merge -p '{"metadata":{"annotations":{"argocd.argoproj.io/refresh":"normal"}}}' \
    2>/dev/null || true
}

# ============================================================================
# Summary Functions
# ============================================================================

print_configuration() {
  print_header "ArgoCD Cache Invalidation Benchmark Setup"
  echo ""
  print_info "Configuration:"
  print_info "  Cluster: $CLUSTER_NAME"
  print_info "  Namespaces: $NAMESPACE_COUNT"
  print_info "  Feature enabled: $FEATURE_ENABLED"
  print_info "  Skip cluster recreate: $SKIP_CLUSTER_RECREATE"
  print_info "  Configure proxy: $CONFIGURE_PROXY"
  print_info "  Configure CA bundle: $CONFIGURE_CA_BUNDLE"
  echo ""
}

print_summary() {
  local password
  password=$(get_secret_value "argocd-initial-admin-secret" "$ARGOCD_NAMESPACE" "{.data.password}")

  echo ""
  print_header "Setup Complete!"
  echo ""
  print_info "Access ArgoCD UI:"
  print_info "  URL: https://localhost:8080"
  print_info "  Username: admin"
  print_info "  Password: $password"
  echo ""

  print_info "Cluster status:"
  kubectl get pods -n "$ARGOCD_NAMESPACE"
  echo ""

  print_info "Test namespaces created:"
  kubectl get ns | grep -c test-ns
  echo ""

  print_info "Guestbook application status:"
  kubectl get application guestbook -n "$ARGOCD_NAMESPACE" \
    -o jsonpath='{.status.sync.status}' 2>/dev/null && echo " (sync status)" || echo "  Application starting..."
  echo ""

  print_info "Guestbook resources in test-ns-1:"
  kubectl get deployments,services -n test-ns-1 2>/dev/null || echo "  Syncing..."
  echo ""
  print_info "To trigger cache invalidation:"
  print_info "  1. Open ArgoCD UI at https://localhost:8080"
  print_info "  2. Go to Settings → Clusters"
  print_info "  3. Click on the in-cluster cluster"
  print_info "  4. Click 'Invalidate Cache' button"
  print_info "  5. Watch controller logs: kubectl logs -n argocd statefulset/argocd-application-controller -f"
  echo ""

  print_info "To add a namespace:"
  print_info " ./benchmark/add-namespace.sh test-ns-101"
  echo ""

  print_info "To compare with feature ENABLED:"
  print_info "  FEATURE_ENABLED=true ./benchmark/setup-benchmark.sh"
  print_info " ./benchmark/add-namespace.sh test-ns-102"
  echo ""

  print_info "To tear down:"
  print_info "  ./benchmark/cleanup.sh"
  echo ""
}

# ============================================================================
# Main Execution
# ============================================================================

main() {
  print_configuration
  create_kind_cluster
  install_argocd
  wait_for_argocd_ready
  create_test_namespaces
  setup_port_forward
  configure_cluster_namespaces
  print_summary
}

main
