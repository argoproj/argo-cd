#!/usr/bin/env sh

# Usage: ./add-protoc-checksums.sh 27.2  # use the desired version

set -e
for arch in aarch_64 ppcle_64 s390_64 x86_64; do
  wget "https://github.com/protocolbuffers/protobuf/releases/download/v$1/protoc-$1-linux-$arch.zip" -O "protoc-$1-linux-$arch.zip"
  sha256sum "protoc-$1-linux-$arch.zip" > "protoc-$1-linux-$arch.zip.sha256"
  rm "protoc-$1-linux-$arch.zip"
done

for arch in aarch_64 x86_64; do
  wget "https://github.com/protocolbuffers/protobuf/releases/download/v$1/protoc-$1-osx-$arch.zip" -O "protoc-$1-osx-$arch.zip"
  sha256sum "protoc-$1-osx-$arch.zip" > "protoc-$1-osx-$arch.zip.sha256"
  rm "protoc-$1-osx-$arch.zip"
done