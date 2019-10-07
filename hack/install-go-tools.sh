#!/bin/sh
set -eux

GO111MODULE=on go get github.com/emicklei/go-restful@v2.7.0
GO111MODULE=on go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.19.1
GO111MODULE=on go get github.com/jstemmer/go-junit-report@v0.0.0-20190106144839-af01ea7f8024
