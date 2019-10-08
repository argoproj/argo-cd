#! /usr/bin/env bash

set -x
set -o errexit
set -o nounset
set -o pipefail

swagger version
jq --version

# shellcheck disable=SC2034
GO111MODULE=on

PROJECT_ROOT=$(cd $(dirname ${BASH_SOURCE})/..; pwd)

# collect_swagger gathers swagger files into a subdirectory
collect_swagger() {
    SWAGGER_ROOT="$1"
    EXPECTED_COLLISIONS="$2"
    SWAGGER_OUT="${PROJECT_ROOT}/assets/swagger.json"
    PRIMARY_SWAGGER=`mktemp`
    COMBINED_SWAGGER=`mktemp`

    cat <<EOF > "${PRIMARY_SWAGGER}"
{
  "swagger": "2.0",
  "info": {
    "title": "Consolidate Services",
    "description": "Description of all APIs",
    "version": "version not set"
  },
  "paths": {}
}
EOF

    /bin/rm -f "${SWAGGER_OUT}"

    /usr/bin/find "${SWAGGER_ROOT}" -name '*.swagger.json' -exec /usr/local/bin/swagger mixin -c "${EXPECTED_COLLISIONS}" "${PRIMARY_SWAGGER}" '{}' \+ > "${COMBINED_SWAGGER}"
    /usr/local/bin/jq -r 'del(.definitions[].properties[]? | select(."$ref"!=null and .description!=null).description) | del(.definitions[].properties[]? | select(."$ref"!=null and .title!=null).title)' "${COMBINED_SWAGGER}" > "${SWAGGER_OUT}"

    /bin/rm "${PRIMARY_SWAGGER}" "${COMBINED_SWAGGER}"
}

# clean up generated swagger files (should come after collect_swagger)
clean_swagger() {
    SWAGGER_ROOT="$1"
    /usr/bin/find "${SWAGGER_ROOT}" -name '*.swagger.json' -delete
}

collect_swagger server 30
clean_swagger server
clean_swagger reposerver
clean_swagger controller
