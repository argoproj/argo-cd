#!/bin/bash
set -e

# ============================================================================
# Configuration
# ============================================================================
readonly CLUSTER_SECRET_NAME="cluster-in-cluster"
readonly NEW_NAMESPACE="${1:-test-ns-101}"
readonly ARGOCD_NAMESPACE="argocd"

# Color codes
readonly COLOR_GREEN='\033[0;32m'
readonly COLOR_YELLOW='\033[1;33m'
readonly COLOR_CYAN='\033[0;36m'
readonly COLOR_RESET='\033[0m'

# Temporary file for log capture
readonly LOG_FILE="/tmp/argocd-sync-${NEW_NAMESPACE}-$$.log"

# ============================================================================
# Utility Functions
# ============================================================================

print_header() {
  echo -e "${COLOR_YELLOW}$1${COLOR_RESET}"
}

print_success() {
  echo -e "${COLOR_GREEN}✓ $1${COLOR_RESET}"
}

print_info() {
  echo "$1"
}

print_metric() {
  echo -e "${COLOR_CYAN}$1${COLOR_RESET}"
}

cleanup() {
  # Clean up log file
  rm -f "$LOG_FILE"
}

trap cleanup EXIT

# ============================================================================
# Kubernetes Helper Functions
# ============================================================================

get_secret_value() {
  kubectl get secret "$CLUSTER_SECRET_NAME" -n "$ARGOCD_NAMESPACE" \
    -o jsonpath='{.data.namespaces}' | base64 -d
}

count_namespaces() {
  echo "$1" | awk -F',' '{print NF}'
}

update_cluster_secret() {
  local namespace_list=$1
  kubectl patch secret "$CLUSTER_SECRET_NAME" -n "$ARGOCD_NAMESPACE" \
    --type merge -p "{\"data\":{\"namespaces\":\"$(echo -n "$namespace_list" | base64 -w 0)\"}}"
}

# ============================================================================
# Log Monitoring Functions
# ============================================================================

# Sync mode configuration
get_log_capture_window() {
  local sync_mode=$1
  [ "$sync_mode" = "incremental" ] && echo "15s" || echo "2m"
}

get_wait_duration() {
  local sync_mode=$1
  [ "$sync_mode" = "incremental" ] && echo 3 || echo 90
}

get_sync_patterns() {
  local sync_mode=$1
  local pattern_type=$2  # "start" or "end"

  if [ "$sync_mode" = "incremental" ]; then
    [ "$pattern_type" = "start" ] && echo "Start syncing namespace" || echo "Namespace successfully synced"
  else
    [ "$pattern_type" = "start" ] && echo "Start syncing cluster" || echo "Cluster successfully synced"
  fi
}

detect_sync_mode() {
  # Wait a bit for ArgoCD to react to the secret change and start logging
  sleep 5

  local recent_logs
  recent_logs=$(kubectl logs -n "$ARGOCD_NAMESPACE" statefulset/argocd-application-controller \
    --since=10s 2>/dev/null | grep -E "syncing namespace|syncing cluster")

  # If "syncing namespace" is found, it's incremental mode, otherwise full
  if echo "$recent_logs" | grep -q "syncing namespace"; then
    echo "incremental"
  else
    echo "full"
  fi
}

capture_sync_logs() {
  local sync_mode=$1
  local since_duration
  since_duration=$(get_log_capture_window "$sync_mode")

  kubectl logs -n "$ARGOCD_NAMESPACE" statefulset/argocd-application-controller \
    --since="$since_duration" --timestamps 2>/dev/null | \
    grep -E "syncing cluster|Cluster successfully synced|syncing namespace|Namespace successfully synced" \
    > "$LOG_FILE" 2>/dev/null || true
}

extract_timestamp_from_log() {
  local log_line=$1
  echo "$log_line" | grep -oP '(?<="time":")[^"]+' || echo ""
}

find_log_entry() {
  local pattern=$1
  local sync_mode=$2

  if [ "$sync_mode" = "incremental" ]; then
    grep "$pattern" "$LOG_FILE" 2>/dev/null | grep "$NEW_NAMESPACE" | tail -1
  else
    grep "$pattern" "$LOG_FILE" 2>/dev/null | tail -1
  fi
}

calculate_duration_seconds() {
  local start_time=$1
  local end_time=$2
  local start_epoch end_epoch

  start_epoch=$(date -d "$start_time" +%s 2>/dev/null || echo "0")
  end_epoch=$(date -d "$end_time" +%s 2>/dev/null || echo "0")

  [ "$start_epoch" -eq 0 ] || [ "$end_epoch" -eq 0 ] && echo "N/A" && return
  echo $((end_epoch - start_epoch))
}

wait_for_sync_completion() {
  local sync_mode=$1
  local wait_seconds
  wait_seconds=$(get_wait_duration "$sync_mode")

  print_info "Detected sync mode: $sync_mode"
  print_info "Waiting for sync to complete..."
  sleep "$wait_seconds"
}

find_sync_log_entries() {
  local sync_mode=$1
  local start_pattern end_pattern

  start_pattern=$(get_sync_patterns "$sync_mode" "start")
  end_pattern=$(get_sync_patterns "$sync_mode" "end")

  local start_log end_log
  start_log=$(find_log_entry "$start_pattern" "$sync_mode")
  end_log=$(find_log_entry "$end_pattern" "$sync_mode")

  echo "$start_log|$end_log"
}

report_missing_logs() {
  local sync_mode=$1
  local start_log=$2
  local end_log=$3

  if [ "$sync_mode" = "incremental" ]; then
    print_info "⚠ Warning: Could not find sync logs for namespace $NEW_NAMESPACE"
  else
    print_info "⚠ Warning: Could not find cluster sync logs"
  fi

  print_info "This might mean the sync completed before we started monitoring,"
  print_info "or the sync hasn't happened yet."
  if [ -n "$start_log" ]; then
    print_info "Start log found: yes"
  else
    print_info "Start log found: no"
  fi
  if [ -n "$end_log" ]; then
    print_info "End log found: yes"
  else
    print_info "End log found: no"
  fi
}

extract_timestamps() {
  local start_log=$1
  local end_log=$2
  local start_time end_time

  start_time=$(extract_timestamp_from_log "$start_log")
  end_time=$(extract_timestamp_from_log "$end_log")

  echo "$start_time|$end_time"
}

display_timestamps() {
  local start_time=$1
  local end_time=$2

  print_info "Sync started at: $start_time"
  print_info "Sync completed at: $end_time"
  echo ""
}

display_sync_results() {
  local sync_mode=$1
  local duration=$2

  print_success "Namespace sync completed successfully!"
  echo ""
  print_metric "╔═══════════════════════════════════════════╗"
  print_metric "║       Namespace Sync Performance          ║"
  print_metric "╠═══════════════════════════════════════════╣"
  print_metric "║  Sync Mode: $(printf "%-28s" "$sync_mode")  ║"
  print_metric "║  Namespace: $(printf "%-28s" "$NEW_NAMESPACE")  ║"
  print_metric "║  Duration:  $(printf "%-23s" "${duration}s")       ║"
  print_metric "╚═══════════════════════════════════════════╝"
  echo ""
}

measure_sync_time() {
  local sync_mode
  sync_mode=$(detect_sync_mode)

  wait_for_sync_completion "$sync_mode"

  print_info "Capturing controller logs..."
  capture_sync_logs "$sync_mode"
  echo ""

  local log_entries start_log end_log
  log_entries=$(find_sync_log_entries "$sync_mode")
  start_log="${log_entries%%|*}"
  end_log="${log_entries##*|}"

  if [ -z "$start_log" ] || [ -z "$end_log" ]; then
    report_missing_logs "$sync_mode" "$start_log" "$end_log"
    return 1
  fi

  local timestamps start_time end_time
  timestamps=$(extract_timestamps "$start_log" "$end_log")
  start_time="${timestamps%%|*}"
  end_time="${timestamps##*|}"

  display_timestamps "$start_time" "$end_time"

  local duration
  duration=$(calculate_duration_seconds "$start_time" "$end_time")

  if [ "$duration" = "N/A" ]; then
    print_info "⚠ Warning: Could not calculate duration"
    return 1
  fi

  display_sync_results "$sync_mode" "$duration"
}

# ============================================================================
# Main Functions
# ============================================================================

create_namespace() {
  print_info "Creating namespace..."
  kubectl create namespace "$NEW_NAMESPACE"
  print_success "Namespace created"
  echo ""
}

update_cluster_configuration() {
  print_info "Getting current namespace list from cluster secret..."
  local current_namespaces
  current_namespaces=$(get_secret_value)
  local namespace_count
  namespace_count=$(count_namespaces "$current_namespaces")
  print_info "Current namespaces: $namespace_count"
  echo ""

  print_info "Adding $NEW_NAMESPACE to cluster configuration..."
  local new_namespace_list="${current_namespaces},${NEW_NAMESPACE}"

  update_cluster_secret "$new_namespace_list"
  print_success "Cluster secret updated"
  echo ""

  local new_count
  new_count=$(count_namespaces "$new_namespace_list")
  print_info "Updated namespace count: $new_count"
  echo ""
}

# ============================================================================
# Main Execution
# ============================================================================

main() {
  print_header "Adding namespace: $NEW_NAMESPACE"
  echo ""

  create_namespace
  update_cluster_configuration

  print_success "Namespace added to cluster configuration!"
  echo ""

  # Measure sync time
  if measure_sync_time; then
    print_info "Sync timing measurement completed successfully"
  else
    print_info "Note: Sync timing measurement had issues, but namespace was added"
  fi

  echo ""
  print_info "To monitor controller logs manually:"
  print_info "  kubectl logs -n argocd statefulset/argocd-application-controller -f"
  echo ""
}

main
