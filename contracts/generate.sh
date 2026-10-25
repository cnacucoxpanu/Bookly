#!/bin/sh
# Regenerates the Go code for every .proto file. Needs protoc,
# protoc-gen-go and protoc-gen-go-grpc in PATH.
set -e
cd "$(dirname "$0")"
protoc \
  --proto_path=proto \
  --go_out=gen --go_opt=module=bookly/contracts/gen \
  --go-grpc_out=gen --go-grpc_opt=module=bookly/contracts/gen \
  proto/auth/v1/*.proto
