#!/bin/sh

# Simple helper script to kill all running processes in the container that
# have belong to the root user.

# DO NOT RUN OUTSIDE THE DOCKER CONTAINER EXECUTING ARGO CD TESTS.
# YOU HAVE BEEN WARNED.

somefunc() {
	echo "Killing all processes"
	sudo pkill -u root
}

echo "Running as $0 ($PWD)"
if test "${PWD}" != "/go/src/github.com/argoproj/argo-cd"; then
	echo "ERROR: We don't seem to be in Docker container. Exit." >&2
	exit 1
fi
trap somefunc 2 15
while :; do
	sleep 1
done
