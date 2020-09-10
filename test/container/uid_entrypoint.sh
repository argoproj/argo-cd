#!/bin/bash
set -e

export PATH=$PATH:/usr/local/go/bin:/go/bin
export GOROOT=/usr/local/go

if test "$(id -u)" == "0" -a "${USER_ID}" != ""; then
  useradd -u ${USER_ID} -d /home/user -s /bin/bash ${USER_NAME:-default}
  chown -R "${USER_NAME:-default}" ${GOCACHE}
  echo "${USER_NAME:-default} ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/${USER_NAME:-default}
  exec sudo -u ${USER_NAME:-default} "$@"
else
  exec "$@"
fi


