#!/usr/bin/env bash
# On macOS with Rancher Desktop managing Docker, containers that share data
# with the host through /tmp - such as the e2e git-server, which serves test
# repos from a bind-mounted /tmp/argo-e2e - can see a stale or incomplete
# view of files the host just wrote. Rancher Desktop's default Lima VM
# sharing mode for /tmp isn't guaranteed to be read-after-write consistent,
# which surfaces as intermittent "repository not found" / "no such file or
# directory" e2e failures. See docs/developer-guide/test-e2e.md.
#
# This is a best-effort nudge towards the documented fix: it never restarts
# Rancher Desktop itself (disruptive, and the contributor's call to make) and
# never touches an existing override.yaml the contributor has already
# customized - it only creates one when none exists yet. Rancher Desktop only
# applies Lima config at VM boot, so a change here needs a restart before it
# takes effect; since e2e failures caused by a stale, not-yet-applied config
# are confusing to debug, this fails the build with clear directions instead
# of letting a contributor run straight into them.

set -u

[ "$(uname -s)" = "Darwin" ] || exit 0
command -v docker >/dev/null 2>&1 || exit 0
[ "$(docker context show 2>/dev/null)" = "rancher-desktop" ] || exit 0

override_file="$HOME/Library/Application Support/rancher-desktop/lima/_config/override.yaml"
rdctl="$HOME/.rd/bin/rdctl"

needs_restart() {
	cat >&2 <<EOF
Attention: Rancher Desktop is managing Docker on this macOS. 
   The file:
   $override_file
   $1
   e2e tests rely on /private/tmp being shared as writable for reliable
   git-server access (see docs/developer-guide/test-e2e.md). Restart
   Rancher Desktop for this to take effect, then re-run this command.
     mounts:
       - location: /private/tmp
         writable: true
EOF
	exit 1
}

if [ ! -f "$override_file" ]; then
	mkdir -p "$(dirname "$override_file")"
	cat >"$override_file" <<'EOF'
mounts:
  - location: /private/tmp
    writable: true
EOF
	needs_restart "was just created with:"
fi

grep -q "/private/tmp" "$override_file" 2>/dev/null || needs_restart "exists but doesn't appear to share /private/tmp as writable. Add this to it:"

# The file already has the mount we need, but it may have been written after
# the VM currently running was booted, in which case the change hasn't been
# picked up yet. Compare the VM's boot time (now minus its uptime, read from
# inside the VM itself, since all Lima config takes effect only at boot)
# against the file's last-modified time.
if [ -x "$rdctl" ]; then
	uptime_seconds=$("$rdctl" shell cat /proc/uptime 2>/dev/null | cut -d. -f1)
	file_mtime=$(stat -f %m "$override_file" 2>/dev/null)
	if [ -n "${uptime_seconds:-}" ] && [ -n "${file_mtime:-}" ]; then
		vm_boot_time=$(($(date +%s) - uptime_seconds))
		if [ "$file_mtime" -gt "$vm_boot_time" ]; then
			needs_restart "was last updated after Rancher Desktop's VM booted, so the change hasn't taken effect yet. Its content is already correct:"
		fi
	fi
fi

exit 0
