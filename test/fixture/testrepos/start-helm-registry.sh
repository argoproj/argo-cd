#!/usr/bin/env bash
export HELM_EXPERIMENTAL_OCI=1
docker run -it -p 5000:5000 --restart=always --name registry registry
