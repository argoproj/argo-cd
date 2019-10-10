#!/bin/bash
set -eux -o pipefail

cd $DOWNLOADS
GO111MODULE=on go get golang.org/x/tools/cmd/goimports@v0.0.0-20190627203933-19ff4fff8850
