#!/usr/bin/env bash

docker run --name e2e-git --rm -i \
    -p 2222:2222 -p 9080:9080 -p 9443:9443 -p 9444:9444 -p 9445:9445 \
    -w /go/src/github.com/argoproj/argo-cd -v $(pwd):/go/src/github.com/argoproj/argo-cd -v /tmp:/tmp docker.io/argoproj/argo-cd-ci-builder:v1.0.0 \
    bash -c "goreman -f ./test/fixture/testrepos/Procfile start"
