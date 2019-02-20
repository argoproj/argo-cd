#!/usr/bin/env bash
set -eu

STAGED_GO_FILES=$(git diff --cached --name-only | grep ".go$")

if [[ "${STAGED_GO_FILES}" != "" ]]; then
    echo "Formatting imports"
    goimports -w ${STAGED_GO_FILES} ;

    echo "Formatting code"
    gofmt -w ${STAGED_GO_FILES} ;

    echo "Linting code"
    gometalinter.v2 --config gometalinter.json ./...
fi
