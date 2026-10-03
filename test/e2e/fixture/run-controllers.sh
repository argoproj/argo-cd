#!/usr/bin/env bash

set -euo pipefail

IFS=' ' read -r -a controller_command <<< "$1"
shift

if [[ -z "${ARGOCD_E2E_CONTROLLER_SHARDS:-}" ]]; then
	exec "${controller_command[@]}" "$@"
fi

IFS=',' read -r -a shards <<< "$ARGOCD_E2E_CONTROLLER_SHARDS"
metrics_port_base="${ARGOCD_E2E_CONTROLLER_METRICS_PORT_BASE:-18082}"
log_dir="${ARGOCD_E2E_CONTROLLER_LOG_DIR:-/tmp}"
pids=()

cleanup() {
	if ((${#pids[@]} > 0)); then
		kill "${pids[@]}" 2>/dev/null || true
		wait "${pids[@]}" 2>/dev/null || true
	fi
}
trap cleanup EXIT
trap 'exit 0' INT TERM

for shard in "${shards[@]}"; do
	log_file="${log_dir}/argocd-e2e-controller-shard-${shard}.log"
	rm -f "$log_file"
	(
		export ARGOCD_CONTROLLER_SHARD="$shard"
		exec "${controller_command[@]}" "$@" --metrics-port="$((metrics_port_base + shard))"
	) >"$log_file" 2>&1 &
	pids+=("$!")
done

status=0
for pid in "${pids[@]}"; do
	if ! wait "$pid"; then
		status=1
	fi
done
exit "$status"
