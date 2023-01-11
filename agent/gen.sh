#! /usr/bin/env nix-shell
#! nix-shell -i bash -p bash protoc-gen-go protoc-gen-go-grpc grpc-tools

### Generate go code from protobuf
set -euo pipefail

rm -f *.pb.go
protoc --plugin $(which protoc-gen-go-grpc) --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative agent.proto
