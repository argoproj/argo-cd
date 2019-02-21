#! /bin/sh
set -eu

STAGED_GO_FILES=$(git diff --cached --name-only | grep ".go$" || true)

if [[ "${STAGED_GO_FILES}" != "" ]]; then
    echo "Formatting imports"
    goimports -w ${STAGED_GO_FILES} ;

    echo "Formatting code"
    gofmt -w ${STAGED_GO_FILES} ;
else
    echo "No staged files to format"
fi
