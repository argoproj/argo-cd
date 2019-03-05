#! /bin/sh
set -eu

CHANGED_GO_FILES=""
for file in $(git diff --name-only | grep ".go$" || true); do
    if [[ -f ${file} ]] ; then
        CHANGED_GO_FILES="${CHANGED_GO_FILES} ${file}"
    fi
done

if [[ "${CHANGED_GO_FILES}" != "" ]]; then
    echo "Formatting imports"
    goimports -w -local github.com/argoproj/argo-cd ${CHANGED_GO_FILES} ;

    echo "Formatting code"
    gofmt -w ${CHANGED_GO_FILES} ;
else
    echo "No changed files to format"
fi
