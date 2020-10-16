#!/bin/bash
set -eux -o pipefail

GO111MODULE=on go get github.com/gogo/protobuf/gogoproto@v1.3.1
GO111MODULE=on go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.3
GO111MODULE=on go get github.com/golang/protobuf/protoc-gen-go@v1.4.2
GO111MODULE=on go get github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway@v1.12.2
GO111MODULE=on go get github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger@v1.12.2
GO111MODULE=on go get golang.org/x/tools/cmd/goimports@v0.0.0-20190627203933-19ff4fff8850
