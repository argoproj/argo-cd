#!/bin/bash
set -e

if test "$(id -u)" == "0" -a "${USER_ID}" != ""; then
  useradd -u ${USER_ID} -d /home/user -s /bin/bash ${USER_NAME:-default}
  chown -R "${USER_NAME:-default}" ${GOCACHE}
fi

export PATH=$PATH:/usr/local/go/bin:/go/bin
export GOROOT=/usr/local/go

if test "$$" = "1"; then
        exec tini -- "$@"
else
        exec "$@"
fi
