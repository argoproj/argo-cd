#!/bin/sh
# This script is used as the command supplied to GIT_ASKPASS as a way to supply username/password
# credentials to git, without having to use git credentials helpers, or having on-disk config.
case "$1" in
Username*) echo "${GIT_USERNAME}" ;;
Password*) echo "${GIT_PASSWORD}" ;;
esac
