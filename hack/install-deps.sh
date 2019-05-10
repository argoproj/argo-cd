#!/usr/bin/env bash
set -eux

go get -u github.com/golang/protobuf/protoc-gen-go
go get -u github.com/go-swagger/go-swagger/cmd/swagger
go get -u github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway
go get -u github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger
go get -u github.com/golangci/golangci-lint/cmd/golangci-lint
go get -u github.com/mattn/goreman
go get -u gotest.tools/gotestsum
