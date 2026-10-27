#!/usr/bin/env bash
# Exercise build-ui's cleanup and copy steps with a small image instead of compiling the UI.
set -euo pipefail

if [ "$(uname -s)" != Linux ] || [ "$(id -u)" -eq 0 ]; then
    echo "Run this test as a non-root Linux user with Docker or rootless Podman access." >&2
    exit 1
fi

repo_dir=$(cd "$(dirname "$0")/.." && pwd)
test_dir=$(mktemp -d)
engine=${DOCKER:-docker}
echo "Testing $engine in $test_dir"
cp "$repo_dir/Makefile" "$repo_dir/VERSION" "$test_dir/"
mkdir -p "$test_dir/ui/dist/app/assets/images/resources"
touch "$test_dir/ui/dist/app/gitkeep" "$test_dir/ui/dist/app/assets/images/resources/.gitkeep"

cat > "$test_dir/Dockerfile" <<'EOF'
FROM --platform=$BUILDPLATFORM docker.io/library/alpine:3.20 AS argocd-ui
WORKDIR /src
RUN mkdir -p dist/app/assets/scripts && \
    echo bundle > dist/app/assets/scripts/bundle.js && \
    echo index > dist/app/index.html
EOF

# Override Go and Git metadata unrelated to the UI build.
for run in 1 2; do
    echo "build-ui run $run"
    make -C "$test_dir" build-ui DOCKER="$engine" \
        TARGET_ARCH="${TARGET_ARCH:-linux/amd64}" \
        HOST_OS=linux HOST_ARCH=amd64 GOPATH="$test_dir/go" \
        GIT_COMMIT=test GIT_TAG=test GIT_TREE_STATE=clean BUILD_DATE=test KUBECTL_VERSION=test
    if [ "$run" -eq 1 ]; then
        touch "$test_dir/ui/dist/app/obsolete.js"
    fi
done

test "$(cat "$test_dir/ui/dist/app/assets/scripts/bundle.js")" = bundle
test "$(cat "$test_dir/ui/dist/app/index.html")" = index
test ! -e "$test_dir/ui/dist/app/obsolete.js"
test -f "$test_dir/ui/dist/app/gitkeep"
test -f "$test_dir/ui/dist/app/assets/images/resources/.gitkeep"
wrong_owners=$(find "$test_dir/ui/dist" \( ! -user "$(id -u)" -o ! -group "$(id -g)" \) -print)
if [ -n "$wrong_owners" ]; then
    echo "Build output must belong to the invoking user and group:" >&2
    echo "$wrong_owners" >&2
    exit 1
fi

rm -rf "$test_dir"
echo "$engine: consecutive builds preserve output ownership and placeholders"
