#!/bin/bash
# Verifies that the tools installed in the local development environment match
# the versions pinned for CI in hack/tool-versions.sh and hack/installers/*.sh.
# On mismatch, prints every matching binary in the PATH to help debug
# discrepancies caused by PATH ordering or stale installations.
set -u -o pipefail

PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")"/.. && pwd) || exit 1

# shellcheck source=hack/tool-versions.sh
. "$PROJECT_ROOT/hack/tool-versions.sh"

# pinned_version <file> <variable> extracts a version pinned in an installer
# script and fails loudly if the variable cannot be parsed, so that a broken
# extraction can never turn into a false OK.
pinned_version() {
    file=$1
    var=$2
    value=$(sed -n "s/^${var}=//p" "$file")
    case "$value" in
        '' | *[[:space:]]*)
            echo "ERROR: could not parse ${var} from ${file}" >&2
            exit 1
            ;;
    esac
    printf '%s\n' "$value"
}

# These versions are pinned in the installer scripts instead of tool-versions.sh.
GOTESTSUM_VERSION=$(pinned_version "$PROJECT_ROOT/hack/installers/install-gotestsum.sh" GOTESTSUM_VERSION) || exit 1
GOLANGCI_LINT_VERSION=$(pinned_version "$PROJECT_ROOT/hack/installers/install-lint-tools.sh" GOLANGCI_LINT_VERSION) || exit 1
MOCKERY_VERSION=$(pinned_version "$PROJECT_ROOT/hack/installers/install-codegen-go-tools.sh" MOCKERY_VERSION) || exit 1

# CI and the codegen scripts put the dist directory first in the PATH.
PATH="$PROJECT_ROOT/dist:$PATH"

failures=0

# check_tool <name> <expected version> <command to print the installed version>
check_tool() {
    name=$1
    expected=$2
    shift 2

    if ! command -v "$name" >/dev/null 2>&1; then
        echo "FAIL ${name}: not found in PATH (expected version: ${expected})"
        failures=$((failures + 1))
        return
    fi

    actual=$("$@" 2>&1)
    if printf '%s' "$actual" | grep -qF "$expected"; then
        echo "OK   ${name} ${expected} ($(command -v "$name"))"
    else
        echo "FAIL ${name}: expected version ${expected}, got:"
        printf '%s\n' "$actual" | sed 's/^/       /'
        echo "     binaries found in PATH:"
        type -a "$name" | sed 's/^/       /'
        failures=$((failures + 1))
    fi
}

check_tool kustomize "$KUSTOMIZE_VERSION" kustomize version
check_tool helm "$HELM_VERSION" helm version
check_tool gotestsum "$GOTESTSUM_VERSION" gotestsum --version
check_tool oras "$oras_version" oras version
check_tool protoc "$protoc_version" protoc --version
check_tool golangci-lint "$GOLANGCI_LINT_VERSION" golangci-lint version
check_tool mockery "$MOCKERY_VERSION" mockery version

if [ "$failures" -gt 0 ]; then
    echo "ERROR: ${failures} tool(s) missing or out of sync with CI."
    echo "Missing tools can be installed with 'make install-tools-local'."
    echo "A version mismatch usually means a stale binary earlier in your PATH shadows the pinned one:"
    echo "the installers write to ${PROJECT_ROOT}/dist or \$BIN (default /usr/local/bin), so check the"
    echo "paths listed above and remove the stale binary or reorder your PATH, then re-run this check."
    exit 1
fi

echo "All tools match the versions used in CI."
