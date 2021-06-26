#!/usr/bin/env bash
export HELM_EXPERIMENTAL_OCI=1
docker run --rm -p 5000:5000 --name registry registry
