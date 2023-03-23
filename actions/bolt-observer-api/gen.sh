#! /usr/bin/env bash

### Generate go code from protobuf
set -euo pipefail

protoc --plugin $(which protoc-gen-go-grpc) --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative actions-api.proto
