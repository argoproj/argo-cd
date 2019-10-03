#!/bin/bash

# Make sure that if we are using an arbitrary UID that it appears in /etc/passwd,
# otherwise this will cause issues with things like cloning with git+ssh
# reference: https://access.redhat.com/documentation/en-us/openshift_container_platform/3.11/html/creating_images/creating-images-guidelines#use-uid
if ! whoami &> /dev/null; then
  if [ -w /etc/passwd ]; then
    echo "${USER_NAME:-default}:x:$(id -u):0:${USER_NAME:-default} user:/home/argocd:/sbin/nologin" >> /etc/passwd
  fi
fi

exec "$@"
