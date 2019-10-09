#!/bin/bash
set -eux -o pipefail

cd $DOWNLOADS
GO111MODULE=on go get github.com/jstemmer/go-junit-report@v0.9.1