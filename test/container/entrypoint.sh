#!/bin/bash
set -e

export PATH=$PATH:/usr/local/go/bin:/go/bin
export GOROOT=/usr/local/go

if test "$$" = "1"; then
        exec tini -- "$@"
else
        exec "$@"
fi
