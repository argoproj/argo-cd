#!/usr/bin/env bash
export HELM_EXPERIMENTAL_OCI=1
$DOCKER run -p 5000:5000 --rm --name registry registry
